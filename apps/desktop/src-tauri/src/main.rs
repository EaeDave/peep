// Peep — Windows tray client. The UI is the bundled apps/web build.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{SystemTime, UNIX_EPOCH};

use tauri::menu::{CheckMenuItem, Menu, MenuItem};
use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent};
use tauri::{AppHandle, Manager, WebviewUrl, WebviewWindow, WebviewWindowBuilder, WindowEvent};
use tauri_plugin_autostart::{MacosLauncher, ManagerExt};
use tauri_plugin_positioner::{Position, WindowExt};

const POPUP: &str = "popup";
const VIEWER: &str = "viewer";

/// When the popup was last hidden by losing focus. Clicking the tray icon
/// blurs the popup first; without this the click would re-open it at once.
static BLUR_HIDDEN_AT: AtomicU64 = AtomicU64::new(0);

fn now_ms() -> u64 {
    SystemTime::now().duration_since(UNIX_EPOCH).map_or(0, |d| d.as_millis() as u64)
}

fn show_popup(app: &AppHandle) {
    let Some(w) = app.get_webview_window(POPUP) else { return };
    // Tray position is only known after a tray event; fall back to the corner.
    if w.as_ref().window().move_window(Position::TrayBottomCenter).is_err() {
        let _ = w.as_ref().window().move_window(Position::BottomRight);
    }
    let _ = w.show();
    let _ = w.set_focus();
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
fn open_viewer(app: AppHandle, code: String) -> Result<(), String> {
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
    let v = WebviewWindowBuilder::new(&app, VIEWER, WebviewUrl::App(format!("r/{code}").into()))
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

fn main() {
    tauri::Builder::default()
        // WIN-6: launching again just shows the existing popup.
        .plugin(tauri_plugin_single_instance::init(|app, _, _| show_popup(app)))
        .plugin(tauri_plugin_autostart::init(MacosLauncher::LaunchAgent, None))
        .plugin(tauri_plugin_positioner::init())
        .invoke_handler(tauri::generate_handler![open_viewer])
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

            let popup = app.get_webview_window(POPUP).expect("popup window in tauri.conf.json");
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
            Ok(())
        })
        // WIN-8: closing the viewer only closes that window (leaving the room);
        // the hidden popup keeps the app alive in the tray.
        .run(tauri::generate_context!())
        .expect("error while running Peep");
}
