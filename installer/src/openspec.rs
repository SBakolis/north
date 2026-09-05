use anyhow::{Context, Result, bail, ensure};
use std::{io::ErrorKind, process::Command};

fn installed_version() -> Result<Option<String>> {
    let output = match Command::new("openspec").arg("--version").output() {
        Ok(output) => output,
        Err(error) if error.kind() == ErrorKind::NotFound => return Ok(None),
        Err(error) => return Err(error).context("Checking OpenSpec; no installation attempted"),
    };
    ensure!(
        output.status.success(),
        "Existing openspec --version failed ({}): {}. Fix the existing CLI before retrying",
        output.status,
        String::from_utf8_lossy(&output.stderr).trim()
    );
    let version = String::from_utf8(output.stdout).context("Reading OpenSpec version")?;
    ensure!(
        !version.trim().is_empty(),
        "openspec --version returned no version"
    );
    Ok(Some(version.trim().to_owned()))
}

fn supported_node(version: &str) -> bool {
    let Some(version) = version.trim().strip_prefix('v') else {
        return false;
    };
    let parts: Vec<_> = version.split('.').map(str::parse::<u32>).collect();
    matches!(parts.as_slice(), [Ok(major), Ok(minor), Ok(patch)] if (*major, *minor, *patch) >= (20, 19, 0))
}

pub fn ensure_installed() -> Result<()> {
    if let Some(version) = installed_version()? {
        println!("OpenSpec is already installed ({version}); skipping installation.");
        return Ok(());
    }
    let node = Command::new("node")
        .arg("--version")
        .output()
        .context("OpenSpec requires Node.js 20.19.0+ and npm on PATH; install them and retry")?;
    ensure!(
        node.status.success() && supported_node(&String::from_utf8_lossy(&node.stdout)),
        "OpenSpec requires Node.js 20.19.0+; install a supported Node.js version and retry"
    );
    println!("Installing OpenSpec: npm install -g @fission-ai/openspec@latest");
    let status = Command::new("npm")
        .args(["install", "-g", "@fission-ai/openspec@latest"])
        .status()
        .context(
            "Running npm; ensure npm is on PATH and its global install directory is writable",
        )?;
    ensure!(
        status.success(),
        "OpenSpec installation failed ({status}); resolve the npm error and retry"
    );
    match installed_version()? {
        Some(version) => println!("OpenSpec installed ({version})."),
        None => bail!(
            "npm completed, but openspec is not on PATH; add npm's global bin directory to PATH and retry"
        ),
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn node_requirement_handles_boundary_and_invalid_versions() {
        for version in ["v20.19.0\n", "v20.19.1", "v22.0.0", "v24.1.0"] {
            assert!(supported_node(version), "{version}");
        }
        for version in [
            "v18.20.0",
            "v20.18.9",
            "v20.19",
            "v20.19.0-rc.1",
            "garbage",
            "",
        ] {
            assert!(!supported_node(version), "{version}");
        }
    }
}
