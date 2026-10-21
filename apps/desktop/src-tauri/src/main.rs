// Peep — Windows tray client. The UI is the bundled apps/web build.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::Mutex;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use tauri::ipc::CapabilityBuilder;
use tauri::menu::{CheckMenuItem, Menu, MenuItem};
use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent};
use tauri::{
    AppHandle, Emitter, Manager, PhysicalPosition, WebviewUrl, WebviewWindow, WebviewWindowBuilder,
    WindowEvent,
};
use tauri_plugin_autostart::{MacosLauncher, ManagerExt};
use tauri_plugin_positioner::{Position, WindowExt};
use tauri_plugin_updater::{Update, UpdaterExt};

mod audio;

const POPUP: &str = "popup";
const VIEWER: &str = "viewer";

/// When the popup was last hidden by losing focus. Clicking the tray icon
/// blurs the popup first; without this the click would re-open it at once.
static BLUR_HIDDEN_AT: AtomicU64 = AtomicU64::new(0);
/// Placed next to the tray once; afterwards the popup stays where the user dragged it.
static PLACED: AtomicBool = AtomicBool::new(false);

const UPDATE_EVERY: Duration = Duration::from_secs(5 * 60 * 60);

/// Update found by the last check, waiting for the user to install it.
#[derive(Default)]
struct PendingUpdate(Mutex<Option<Update>>);

fn now_ms() -> u64 {
    SystemTime::now().duration_since(UNIX_EPOCH).map_or(0, |d| d.as_millis() as u64)
}

/// Keeps the popup inside the monitor's work area (above/beside the taskbar).
fn clamp_to_screen(w: &WebviewWindow) {
    let (Ok(Some(m)), Ok(pos), Ok(size)) = (w.current_monitor(), w.outer_position(), w.outer_size())
    else {
        return;
    };
    let area = m.work_area();
    let max_x = area.position.x + area.size.width as i32 - size.width as i32;
    let max_y = area.position.y + area.size.height as i32 - size.height as i32;
    let x = pos.x.clamp(area.position.x, max_x.max(area.position.x));
    let y = pos.y.clamp(area.position.y, max_y.max(area.position.y));
    if (x, y) != (pos.x, pos.y) {
        let _ = w.set_position(PhysicalPosition::new(x, y));
    }
}

fn show_popup(app: &AppHandle) {
    let Some(w) = app.get_webview_window(POPUP) else { return };
    if !PLACED.load(Ordering::Relaxed) {
        // TrayCenter opens above the tray icon (Windows taskbar at the bottom).
        // Tray position is only known after a tray event; fall back to the corner.
        if w.as_ref().window().move_window(Position::TrayCenter).is_ok() {
            PLACED.store(true, Ordering::Relaxed);
        } else {
            let _ = w.as_ref().window().move_window(Position::BottomRight);
        }
    }
    clamp_to_screen(&w);
    let _ = w.show();
    let _ = w.set_focus();
}

/// Checks the release feed; caches the update and notifies the popup.
async fn find_update(app: &AppHandle) -> Result<Option<String>, String> {
    let update = app.updater().map_err(|e| e.to_string())?.check().await.map_err(|e| e.to_string())?;
    let version = update.as_ref().map(|u| u.version.clone());
    *app.state::<PendingUpdate>().0.lock().unwrap() = update;
    if let Some(v) = &version {
        let _ = app.emit("update-available", v);
        if let Some(tray) = app.tray_by_id("peep") {
            let _ = tray.set_tooltip(Some(format!("Peep — versão {v} disponível")));
        }
    }
    Ok(version)
}

fn current_version(app: &AppHandle) -> String {
    app.package_info().version.to_string()
}

/// Current version and the pending update found by the background check.
#[tauri::command]
fn update_status(app: AppHandle) -> (String, Option<String>) {
    let pending = app.state::<PendingUpdate>().0.lock().unwrap().as_ref().map(|u| u.version.clone());
    (current_version(&app), pending)
}

#[tauri::command]
async fn check_update(app: AppHandle) -> Result<(String, Option<String>), String> {
    let available = find_update(&app).await?;
    Ok((current_version(&app), available))
}

/// Downloads, verifies the signature, installs and restarts. Never during a room.
#[tauri::command]
async fn install_update(app: AppHandle) -> Result<(), String> {
    if app.get_webview_window(VIEWER).is_some() {
        return Err("Saia da sala antes de atualizar.".into());
    }
    let update = app.state::<PendingUpdate>().0.lock().unwrap().take();
    let update = update.ok_or("Nenhuma atualização disponível.")?;
    update.download_and_install(|_, _| {}, || {}).await.map_err(|e| e.to_string())?;
    app.restart();
}

fn toggle_popup(app: &AppHandle) {
    let Some(w) = app.get_webview_window(POPUP) else { return };
    if w.is_visible().unwrap_or(false) {
        let _ = w.hide();
    } else if now_ms() - BLUR_HIDDEN_AT.load(Ordering::Relaxed) > 300 {
        show_popup(app);
    }
}

/// WIN-5: grant camera/mic/screen capture to our own bundled pages, no prompts.
#[cfg(windows)]
fn grant_media(w: &WebviewWindow) {
    use webview2_com::Microsoft::Web::WebView2::Win32::{
        COREWEBVIEW2_PERMISSION_KIND_CAMERA, COREWEBVIEW2_PERMISSION_KIND_MICROPHONE,
        COREWEBVIEW2_PERMISSION_STATE_ALLOW,
    };
    use webview2_com::PermissionRequestedEventHandler;

    let _ = w.with_webview(|wv| unsafe {
        let Ok(core) = wv.controller().CoreWebView2() else { return };
        let handler = PermissionRequestedEventHandler::create(Box::new(|_, args| {
            if let Some(args) = args {
                let mut kind = Default::default();
                args.PermissionKind(&mut kind)?;
                if kind == COREWEBVIEW2_PERMISSION_KIND_CAMERA
                    || kind == COREWEBVIEW2_PERMISSION_KIND_MICROPHONE
                {
                    args.SetState(COREWEBVIEW2_PERMISSION_STATE_ALLOW)?;
                }
            }
            Ok(())
        }));
        let mut token = Default::default();
        let _ = core.add_PermissionRequested(&handler, &mut token);
    });
}

#[cfg(not(windows))]
fn grant_media(_: &WebviewWindow) {}

/// WIN-3: create or focus the viewer window on /r/{code}; hides the popup.
#[tauri::command]
// Must be async: creating a window from a sync command deadlocks WebView2 on
// Windows (blank window; tauri-apps/wry#583).
async fn open_viewer(app: AppHandle, code: String) -> Result<(), String> {
    let code: String = code
        .chars()
        .filter(|c| c.is_ascii_alphanumeric() || *c == '-')
        .take(7)
        .collect();
    if let Some(p) = app.get_webview_window(POPUP) {
        let _ = p.hide();
    }
    if let Some(v) = app.get_webview_window(VIEWER) {
        let mut url = v.url().map_err(|e| e.to_string())?;
        url.set_path(&format!("/r/{code}"));
        url.set_query(None);
        v.navigate(url).map_err(|e| e.to_string())?;
        let _ = v.set_title(&format!("Peep — {code}"));
        let _ = v.unminimize();
        let _ = v.show();
        let _ = v.set_focus();
        return Ok(());
    }
    let v = WebviewWindowBuilder::new(&app, VIEWER, page(&format!("r/{code}")))
        .title(format!("Peep — {code}"))
        .inner_size(1280.0, 720.0)
        .min_inner_size(480.0, 270.0)
        .resizable(true)
        .center()
        .build()
        .map_err(|e| e.to_string())?;
    grant_media(&v);
    Ok(())
}

/// Site the windows load (PEEP_WEB_URL at build time), so UI changes ship with
/// the site deploy like the Omarchy client. Unset → the bundled frontend.
fn site() -> Option<&'static str> {
    option_env!("PEEP_WEB_URL").map(|u| u.trim_end_matches('/')).filter(|u| !u.is_empty())
}

fn page(path: &str) -> WebviewUrl {
    match site().and_then(|base| format!("{base}/{path}").parse().ok()) {
        Some(url) => WebviewUrl::External(url),
        None => WebviewUrl::App(path.into()),
    }
}

fn main() {
    tauri::Builder::default()
        // WIN-6: launching again just shows the existing popup.
        .plugin(tauri_plugin_single_instance::init(|app, _, _| show_popup(app)))
        .plugin(tauri_plugin_autostart::init(MacosLauncher::LaunchAgent, None))
        .plugin(tauri_plugin_positioner::init())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .manage(PendingUpdate::default())
        .manage(audio::ScreenAudio::default())
        .invoke_handler(tauri::generate_handler![
            open_viewer,
            update_status,
            check_update,
            install_update,
            audio::start_screen_audio,
            audio::stop_screen_audio
        ])
        .setup(|app| {
            let open = MenuItem::with_id(app, "open", "Abrir", true, None::<&str>)?;
            let autostart = CheckMenuItem::with_id(
                app,
                "autostart",
                "Iniciar com o Windows",
                true,
                app.autolaunch().is_enabled().unwrap_or(false),
                None::<&str>,
            )?;
            let quit = MenuItem::with_id(app, "quit", "Sair", true, None::<&str>)?;
            let menu = Menu::with_items(app, &[&open, &autostart, &quit])?;

            TrayIconBuilder::with_id("peep")
                .icon(app.default_window_icon().cloned().expect("bundle icon"))
                .tooltip("Peep")
                .menu(&menu)
                .show_menu_on_left_click(false)
                .on_menu_event(move |app, e| match e.id.as_ref() {
                    "open" => show_popup(app),
                    "autostart" => {
                        let al = app.autolaunch();
                        let on = al.is_enabled().unwrap_or(false);
                        let _ = if on { al.disable() } else { al.enable() };
                        let _ = autostart.set_checked(al.is_enabled().unwrap_or(false));
                    }
                    "quit" => app.exit(0),
                    _ => {}
                })
                .on_tray_icon_event(|tray, e| {
                    tauri_plugin_positioner::on_tray_event(tray.app_handle(), &e);
                    if let TrayIconEvent::Click {
                        button: MouseButton::Left,
                        button_state: MouseButtonState::Up,
                        ..
                    } = e
                    {
                        toggle_popup(tray.app_handle());
                    }
                })
                .build(app)?;

            // Remote pages get IPC only for this exact site.
            if let Some(base) = site() {
                app.add_capability(
                    CapabilityBuilder::new("site")
                        .remote(format!("{base}/*"))
                        .windows([POPUP, VIEWER])
                        .permission("core:default")
                        .permission("core:window:allow-close")
                        .permission("core:window:allow-start-dragging")
                        .permission("allow-open-viewer")
                        .permission("allow-update-status")
                        .permission("allow-check-update")
                        .permission("allow-install-update")
                        .permission("allow-start-screen-audio")
                        .permission("allow-stop-screen-audio"),
                )?;
            }
            let popup = WebviewWindowBuilder::new(app, POPUP, page("join"))
                .title("Peep")
                .inner_size(320.0, 480.0)
                .resizable(false)
                .decorations(false)
                .always_on_top(true)
                .skip_taskbar(true)
                .visible(false)
                .focused(true)
                .build()?;
            grant_media(&popup);
            let p = popup.clone();
            popup.on_window_event(move |e| match e {
                // WIN-2: the popup hides when it loses focus.
                WindowEvent::Focused(false) => {
                    if p.is_visible().unwrap_or(false) {
                        BLUR_HIDDEN_AT.store(now_ms(), Ordering::Relaxed);
                        let _ = p.hide();
                    }
                }
                // Alt+F4 on the popup hides it; the app lives in the tray.
                WindowEvent::CloseRequested { api, .. } => {
                    api.prevent_close();
                    let _ = p.hide();
                }
                _ => {}
            });

            // Checks for a new release at startup and every 5 hours.
            let handle = app.handle().clone();
            std::thread::spawn(move || loop {
                let _ = tauri::async_runtime::block_on(find_update(&handle));
                std::thread::sleep(UPDATE_EVERY);
            });
            Ok(())
        })
        // WIN-8: closing the viewer only closes that window (leaving the room);
        // the hidden popup keeps the app alive in the tray.
        .run(tauri::generate_context!())
        .expect("error while running Peep");
}
