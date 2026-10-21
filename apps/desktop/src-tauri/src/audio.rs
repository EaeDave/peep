//! Screen-share audio on Windows via WASAPI process loopback (Windows 10 2004+).
//! Shared window → audio of that app's process tree only.
//! Whole screen → every app except Peep itself, so the call never echoes back.
//! PCM (48 kHz, f32 stereo interleaved) streams to the page over an IPC channel.

use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};

use tauri::ipc::{Channel, InvokeResponseBody};
use tauri::State;

/// Stop flag of the running capture thread, if any.
#[derive(Default)]
pub struct ScreenAudio(Mutex<Option<Arc<AtomicBool>>>);

impl ScreenAudio {
    fn stop(&self) {
        if let Some(running) = self.0.lock().unwrap().take() {
            running.store(false, Ordering::Relaxed);
        }
    }
}

/// `hwnd`: native handle of the shared window (from the capture deviceId); none = whole screen.
/// Async: reading the WebView2 process id hops to the main thread.
#[tauri::command]
pub async fn start_screen_audio(
    webview: tauri::Webview,
    state: State<'_, ScreenAudio>,
    hwnd: Option<i64>,
    channel: Channel<InvokeResponseBody>,
) -> Result<(), String> {
    state.stop();
    let call = imp::call_process(&webview);
    let running = Arc::new(AtomicBool::new(true));
    imp::start(hwnd, call, channel, running.clone())?;
    *state.0.lock().unwrap() = Some(running);
    Ok(())
}

#[tauri::command]
pub fn stop_screen_audio(state: State<ScreenAudio>) {
    state.stop();
}

#[cfg(windows)]
mod imp {
    use std::collections::VecDeque;
    use std::sync::atomic::{AtomicBool, Ordering};
    use std::sync::{mpsc, Arc};

    use tauri::ipc::{Channel, InvokeResponseBody};
    use wasapi::{
        initialize_mta, AudioClient, Direction, SampleType, StreamMode, WaveFormat,
    };

    #[link(name = "user32")]
    unsafe extern "system" {
        fn GetWindowThreadProcessId(hwnd: isize, pid: *mut u32) -> u32;
    }

    /// 10 ms at 48 kHz per IPC message.
    const CHUNK_FRAMES: usize = 480;

    /// Process tree that plays the call: the WebView2 browser process, which is
    /// not necessarily a descendant of Peep.exe. Excluding Peep.exe alone let
    /// the other participants' voices back into whole-screen shares.
    pub fn call_process(webview: &tauri::Webview) -> u32 {
        let (tx, rx) = mpsc::channel();
        let asked = webview.with_webview(move |wv| unsafe {
            let mut pid = 0u32;
            let ok = wv.controller().CoreWebView2().and_then(|c| c.BrowserProcessId(&mut pid)).is_ok();
            let _ = tx.send(if ok && pid != 0 { Some(pid) } else { None });
        });
        asked
            .ok()
            .and_then(|_| rx.recv_timeout(std::time::Duration::from_secs(2)).ok().flatten())
            .unwrap_or_else(std::process::id)
    }

    /// (process, include its tree): the shared window's app, or everything but the call.
    fn target(hwnd: Option<i64>, call: u32) -> (u32, bool) {
        if let Some(h) = hwnd {
            let mut pid = 0u32;
            unsafe { GetWindowThreadProcessId(h as isize, &mut pid) };
            if pid != 0 && pid != call && pid != std::process::id() {
                return (pid, true);
            }
        }
        (call, false)
    }

    pub fn start(
        hwnd: Option<i64>,
        call: u32,
        channel: Channel<InvokeResponseBody>,
        running: Arc<AtomicBool>,
    ) -> Result<(), String> {
        let (pid, include_tree) = target(hwnd, call);
        // The thread reports whether WASAPI accepted the client before streaming.
        let (ready_tx, ready_rx) = mpsc::channel();
        std::thread::Builder::new()
            .name("screen-audio".into())
            .spawn(move || {
                if let Err(e) = capture(pid, include_tree, &channel, &running, &ready_tx) {
                    let _ = ready_tx.send(Err(e));
                }
            })
            .map_err(|e| e.to_string())?;
        ready_rx
            .recv()
            .map_err(|_| "A captura de áudio terminou inesperadamente.".to_string())?
    }

    fn capture(
        pid: u32,
        include_tree: bool,
        channel: &Channel<InvokeResponseBody>,
        running: &AtomicBool,
        ready: &mpsc::Sender<Result<(), String>>,
    ) -> Result<(), String> {
        let err = |e: wasapi::WasapiError| e.to_string();
        initialize_mta().ok().map_err(|e| e.to_string())?;
        let format = WaveFormat::new(32, 32, &SampleType::Float, 48_000, 2, None);
        let chunk_bytes = format.get_blockalign() as usize * CHUNK_FRAMES;
        let mut client = AudioClient::new_application_loopback_client(pid, include_tree)
            .map_err(|e| format!("Captura de áudio indisponível (requer Windows 10 2004+): {e}"))?;
        let mode = StreamMode::EventsShared { autoconvert: true, buffer_duration_hns: 0 };
        client.initialize_client(&format, &Direction::Capture, &mode).map_err(err)?;
        let event = client.set_get_eventhandle().map_err(err)?;
        let reader = client.get_audiocaptureclient().map_err(err)?;
        client.start_stream().map_err(err)?;
        let _ = ready.send(Ok(()));

        let mut queue = VecDeque::<u8>::new();
        while running.load(Ordering::Relaxed) {
            // Process loopback signals nothing while the target is silent.
            if event.wait_for_event(100).is_err() {
                continue;
            }
            while reader.get_next_packet_size().map_err(err)?.unwrap_or(0) > 0 {
                reader.read_from_device_to_deque(&mut queue).map_err(err)?;
            }
            while queue.len() >= chunk_bytes {
                let chunk: Vec<u8> = queue.drain(..chunk_bytes).collect();
                // The page is gone (window closed or reloaded): stop capturing.
                if channel.send(InvokeResponseBody::Raw(chunk)).is_err() {
                    running.store(false, Ordering::Relaxed);
                    break;
                }
            }
        }
        client.stop_stream().map_err(err)
    }
}

#[cfg(not(windows))]
mod imp {
    use std::sync::atomic::AtomicBool;
    use std::sync::Arc;

    use tauri::ipc::{Channel, InvokeResponseBody};

    pub fn call_process(_: &tauri::Webview) -> u32 {
        std::process::id()
    }

    pub fn start(_: Option<i64>, _: u32, _: Channel<InvokeResponseBody>, _: Arc<AtomicBool>) -> Result<(), String> {
        Err("Áudio nativo da tela está disponível apenas no Windows.".into())
    }
}
