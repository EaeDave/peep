// App commands get ACL permissions (allow-<command>): required for the remote
// site to call them, and then enforced for local pages too.
const COMMANDS: &[&str] = &["open_viewer", "update_status", "check_update", "install_update"];

fn main() {
    tauri_build::try_build(
        tauri_build::Attributes::new()
            .app_manifest(tauri_build::AppManifest::new().commands(COMMANDS)),
    )
    .expect("tauri build");
}
