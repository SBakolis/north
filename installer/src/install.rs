use crate::config::ConfigMerge;
use anyhow::{Context, Result, bail, ensure};
use serde::{Deserialize, Serialize};
use std::{
    collections::{BTreeMap, BTreeSet},
    fs::{self, OpenOptions},
    io::{ErrorKind, Write},
    os::unix::fs::{OpenOptionsExt, symlink},
    path::{Component, Path, PathBuf},
};

const STATE: &str = ".north-installation.json";
const BACKUP: &str = "AGENTS-backup.md";
const COMMIT: &str = "commit";
pub const AUTO_COMMIT: &str = "auto-commit";

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
struct State {
    version: u32,
    repo: PathBuf,
    backup: bool,
    links: BTreeMap<PathBuf, PathBuf>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    configs: Vec<ConfigMerge>,
}

pub struct Installation {
    pub config: PathBuf,
    pub skills: Vec<String>,
    repo: PathBuf,
    available: BTreeMap<PathBuf, PathBuf>,
    state: Option<State>,
}

fn exists(path: &Path) -> Result<bool> {
    match fs::symlink_metadata(path) {
        Ok(_) => Ok(true),
        Err(error) if error.kind() == ErrorKind::NotFound => Ok(false),
        Err(error) => Err(error).with_context(|| format!("Inspecting {}", path.display())),
    }
}

fn matches_link(path: &Path, source: &Path) -> bool {
    fs::read_link(path).is_ok_and(|target| target == source)
}

fn check_directory(path: &Path) -> Result<()> {
    if exists(path)? {
        ensure!(
            fs::symlink_metadata(path)?.is_dir(),
            "Expected a real directory at {}; move the conflicting file or symlink aside first",
            path.display()
        );
    }
    Ok(())
}

fn read_state(config: &Path) -> Result<Option<State>> {
    let path = config.join(STATE);
    if !exists(&path)? {
        return Ok(None);
    }
    ensure!(
        fs::symlink_metadata(&path)?.is_file(),
        "Expected a regular state file at {}",
        path.display()
    );
    let state: State = serde_json::from_slice(&fs::read(&path)?).with_context(|| {
        format!(
            "Reading {}; preserve this file for recovery",
            path.display()
        )
    })?;
    ensure!(
        matches!(state.version, 1 | 2) && state.repo.is_absolute(),
        "Unsupported North installation state"
    );
    for (relative, source) in &state.links {
        let parts: Vec<_> = relative.components().collect();
        ensure!(
            parts
                .iter()
                .all(|part| matches!(part, Component::Normal(_))),
            "Invalid path in North installation state"
        );
        let expected = if relative == Path::new("AGENTS.md") {
            state.repo.join("assets/instructions/core.md")
        } else {
            ensure!(
                parts.len() == 2
                    && ["skills", "agents", "commands"]
                        .iter()
                        .any(|folder| parts[0].as_os_str() == *folder),
                "Invalid link in North installation state"
            );
            state.repo.join("assets").join(relative)
        };
        ensure!(
            source == &expected,
            "Invalid source in North installation state"
        );
    }
    let mut names = BTreeSet::new();
    for config in &state.configs {
        ensure!(
            ["opencode.json", "opencode.jsonc"].contains(&config.file.as_str())
                && names.insert(&config.file),
            "Invalid configuration file in North installation state"
        );
    }
    ensure!(
        state.links.contains_key(Path::new("AGENTS.md")) || !state.configs.is_empty(),
        "Missing instructions link in North installation state"
    );
    Ok(Some(state))
}

impl Installation {
    pub fn load(repo: &Path, config: &Path) -> Result<Self> {
        let repo = repo.canonicalize().context("Locating North checkout")?;
        check_directory(config)?;
        check_directory(&config.join("agents"))?;
        check_directory(&config.join("skills"))?;
        check_directory(&config.join("commands"))?;
        let mut available = BTreeMap::new();
        let core = repo.join("assets/instructions/core.md");
        ensure!(core.is_file(), "Missing {}", core.display());
        available.insert(PathBuf::from("AGENTS.md"), core);
        let mut skills = Vec::new();
        for folder in ["agents", "skills", "commands"] {
            for entry in fs::read_dir(repo.join("assets").join(folder))? {
                let entry = entry?;
                let path = entry.path();
                let relevant = if folder == "skills" {
                    path.is_dir() && path.join("SKILL.md").is_file()
                } else {
                    path.is_file() && path.extension().is_some_and(|ext| ext == "md")
                };
                if relevant {
                    let name = entry
                        .file_name()
                        .into_string()
                        .map_err(|_| anyhow::anyhow!("Asset names must be UTF-8"))?;
                    if folder == "skills" && name != COMMIT {
                        skills.push(name.clone());
                    }
                    available.insert(Path::new(folder).join(name), path);
                }
            }
        }
        skills.sort();
        ensure!(
            available.contains_key(Path::new("skills/commit"))
                == available.contains_key(Path::new("skills/auto-commit")),
            "The commit and auto-commit skills must be bundled together"
        );
        Ok(Self {
            state: read_state(config)?,
            repo,
            config: config.to_owned(),
            available,
            skills,
        })
    }

    pub fn skill_names(&self) -> BTreeSet<String> {
        self.skills.iter().cloned().collect()
    }

    // Adopt links from the previous shell installer, which had no state file.
    fn owned_links(&self) -> BTreeMap<PathBuf, PathBuf> {
        self.state
            .as_ref()
            .map(|state| state.links.clone())
            .unwrap_or_else(|| {
                self.available
                    .iter()
                    .filter(|(relative, source)| matches_link(&self.config.join(relative), source))
                    .map(|(relative, source)| (relative.clone(), source.clone()))
                    .collect()
            })
    }

    pub fn installed(&self) -> bool {
        self.state.is_some() || !self.owned_links().is_empty()
    }

    pub fn merging(&self) -> bool {
        self.state
            .as_ref()
            .is_some_and(|state| !state.configs.is_empty())
    }

    pub fn selected_skills(&self) -> BTreeSet<String> {
        if !self.installed() {
            return self.skill_names();
        }
        let owned = self.owned_links();
        self.skills
            .iter()
            .filter(|name| {
                let relative = Path::new("skills").join(name);
                owned
                    .get(&relative)
                    .is_some_and(|source| matches_link(&self.config.join(relative), source))
            })
            .cloned()
            .collect()
    }

    fn lock(&self) -> Result<Lock> {
        check_directory(&self.config)?;
        fs::create_dir_all(&self.config)?;
        let path = self.config.join(".north-install.lock");
        fs::create_dir(&path).with_context(|| format!("Cannot lock installation. Another installer may be running; if it crashed, remove {} and retry", path.display()))?;
        let lock = Lock(path);
        ensure!(
            read_state(&self.config)? == self.state,
            "Installation changed while the menu was open; rerun install.sh"
        );
        check_directory(&self.config.join("agents"))?;
        check_directory(&self.config.join("skills"))?;
        check_directory(&self.config.join("commands"))?;
        Ok(lock)
    }

    // The checklist stores auto-commit as one option; commit is its unchecked fallback.
    pub fn resolved_skills(&self, selected: &BTreeSet<String>) -> Result<BTreeSet<String>> {
        let mut known = self.skill_names();
        if self.available.contains_key(Path::new("skills/commit")) {
            known.insert(COMMIT.into());
        }
        ensure!(
            selected.is_subset(&known),
            "Unknown skills: {}",
            selected
                .difference(&known)
                .cloned()
                .collect::<Vec<_>>()
                .join(", ")
        );
        ensure!(
            !(selected.contains(COMMIT) && selected.contains(AUTO_COMMIT)),
            "Choose commit or auto-commit, not both"
        );
        let mut resolved = selected.clone();
        if known.contains(COMMIT) && !selected.contains(AUTO_COMMIT) {
            resolved.insert(COMMIT.into());
        }
        Ok(resolved)
    }

    #[cfg(test)]
    pub fn apply(&self, selected: &BTreeSet<String>) -> Result<()> {
        self.apply_with_merge(selected, self.merging())
    }

    pub fn apply_with_merge(&self, selected: &BTreeSet<String>, merge: bool) -> Result<()> {
        let selected = self.resolved_skills(selected)?;
        let _lock = self.lock()?;
        let owned = self.owned_links();
        // Never leave the opposite commit mode active or remove a user's replacement.
        for name in [COMMIT, AUTO_COMMIT] {
            let relative = Path::new("skills").join(name);
            let target = self.config.join(&relative);
            if self.available.contains_key(&relative)
                && !selected.contains(name)
                && exists(&target)?
            {
                ensure!(
                    owned
                        .get(&relative)
                        .is_some_and(|source| matches_link(&target, source)),
                    "Conflicting commit mode at {}; move it aside before switching modes",
                    target.display()
                );
            }
        }
        let desired: BTreeMap<_, _> = self
            .available
            .iter()
            .filter(|(relative, _)| {
                (!merge || relative.as_path() != Path::new("AGENTS.md"))
                    && (!relative.starts_with("skills")
                        || selected.contains(relative.file_name().unwrap().to_str().unwrap()))
            })
            .map(|(relative, source)| (relative.clone(), source.clone()))
            .collect();
        let agents = self.config.join("AGENTS.md");
        let backup = self.config.join(BACKUP);
        let mut has_backup = self.state.as_ref().is_some_and(|state| state.backup);
        if has_backup {
            ensure!(
                exists(&backup)? && !fs::symlink_metadata(&backup)?.is_dir(),
                "Saved {} is missing or invalid; restore it before changing North",
                backup.display()
            );
        } else {
            ensure!(
                !exists(&backup)?,
                "Refusing to overwrite an untracked {}; move it aside first",
                backup.display()
            );
        }
        let (configs, mut actions) = self.config_changes(merge)?;
        for (relative, source) in &desired {
            let target = self.config.join(relative);
            if matches_link(&target, source) {
                continue;
            }
            if exists(&target)? {
                if owned
                    .get(relative)
                    .is_some_and(|old| matches_link(&target, old))
                {
                    actions.push(Change::Unlink(target.clone(), owned[relative].clone()));
                } else if relative == Path::new("AGENTS.md")
                    && (self.state.is_none() || self.merging())
                {
                    ensure!(
                        !fs::symlink_metadata(&agents)?.is_dir(),
                        "AGENTS.md must be a file or symlink"
                    );
                    actions.push(Change::Rename(agents.clone(), backup.clone()));
                    has_backup = true;
                } else {
                    bail!(
                        "Refusing to replace {}; move the conflicting file or link aside first",
                        target.display()
                    );
                }
            }
            actions.push(Change::Link(target, source.clone()));
        }
        for (relative, source) in &owned {
            if !desired.contains_key(relative) && matches_link(&self.config.join(relative), source)
            {
                actions.push(Change::Unlink(self.config.join(relative), source.clone()));
            }
        }
        if merge && has_backup {
            ensure!(
                owned
                    .get(Path::new("AGENTS.md"))
                    .is_some_and(|source| matches_link(&agents, source))
                    || !exists(&agents)?,
                "AGENTS.md was changed outside North; move it aside before restoring the backup"
            );
            actions.push(Change::Rename(backup, agents));
            has_backup = false;
        }
        let state = State {
            version: if configs.is_empty() { 1 } else { 2 },
            repo: self.repo.clone(),
            backup: has_backup,
            links: desired,
            configs,
        };
        let bytes = serde_json::to_vec_pretty(&state)?;
        fs::create_dir_all(self.config.join("agents"))?;
        fs::create_dir_all(self.config.join("skills"))?;
        fs::create_dir_all(self.config.join("commands"))?;
        transact(&actions, || write_state(&self.config, &bytes))
    }

    fn config_changes(&self, merge: bool) -> Result<(Vec<ConfigMerge>, Vec<Change>)> {
        let previous = self
            .state
            .as_ref()
            .map(|state| state.configs.as_slice())
            .unwrap_or_default();
        if !merge && previous.is_empty() {
            return Ok((Vec::new(), Vec::new()));
        }
        let mut north = if merge {
            let path = self.repo.join("assets/opencode.json");
            serde_json::from_slice::<serde_json::Value>(
                &fs::read(&path).with_context(|| format!("Reading {}", path.display()))?,
            )?
        } else {
            serde_json::json!({})
        };
        ensure!(north.is_object(), "North configuration must be an object");
        if merge {
            let instructions = north
                .as_object_mut()
                .unwrap()
                .entry("instructions")
                .or_insert(serde_json::json!([]));
            let instructions = instructions
                .as_array_mut()
                .context("North instructions must be an array")?;
            let core = serde_json::json!(self.repo.join("assets/instructions/core.md"));
            if !instructions.contains(&core) {
                instructions.push(core);
            }
        }
        let mut names: BTreeSet<String> =
            previous.iter().map(|config| config.file.clone()).collect();
        if merge {
            for name in ["opencode.json", "opencode.jsonc"] {
                if exists(&self.config.join(name))? {
                    names.insert(name.into());
                }
            }
            if names.is_empty() {
                names.insert("opencode.json".into());
            }
        }
        let mut configs = Vec::new();
        let mut actions = Vec::new();
        for name in names {
            let path = self.config.join(&name);
            let before = read_config(&path)?;
            let original = match previous.iter().find(|config| config.file == name) {
                Some(config) => config
                    .remove(before.as_deref())
                    .with_context(|| format!("Updating {}", path.display()))?,
                None => before.clone(),
            };
            let after = if merge {
                let config = ConfigMerge::new(name, original, &north)
                    .with_context(|| format!("Merging {}", path.display()))?;
                let after = Some(config.applied.clone());
                configs.push(config);
                after
            } else {
                original
            };
            if before != after {
                actions.push(Change::Config(path, before, after));
            }
        }
        Ok((configs, actions))
    }

    pub fn uninstall(&self) -> Result<()> {
        let _lock = self.lock()?;
        let owned = self.owned_links();
        let (_, mut actions) = self.config_changes(false)?;
        for (relative, source) in &owned {
            let target = self.config.join(relative);
            if matches_link(&target, source) {
                actions.push(Change::Unlink(target, source.clone()));
            }
        }
        if self.state.as_ref().is_some_and(|state| state.backup) {
            let agents = self.config.join("AGENTS.md");
            let backup = self.config.join(BACKUP);
            ensure!(
                exists(&backup)? && !fs::symlink_metadata(&backup)?.is_dir(),
                "Saved {} is missing or invalid; restore it before uninstalling",
                backup.display()
            );
            ensure!(
                !exists(&agents)?
                    || owned
                        .get(Path::new("AGENTS.md"))
                        .is_some_and(|source| matches_link(&agents, source)),
                "AGENTS.md was changed outside North; move it aside before restoring the backup"
            );
            actions.push(Change::Rename(backup, agents));
        }
        transact(&actions, || {
            if self.state.is_some() {
                fs::remove_file(self.config.join(STATE))?;
            }
            Ok(())
        })?;
        // Only remove empty North destination directories; unrelated files survive.
        for folder in ["agents", "skills", "commands"] {
            let _ = fs::remove_dir(self.config.join(folder));
        }
        Ok(())
    }
}

struct Lock(PathBuf);
impl Drop for Lock {
    fn drop(&mut self) {
        let _ = fs::remove_dir(&self.0);
    }
}

enum Change {
    Link(PathBuf, PathBuf),
    Unlink(PathBuf, PathBuf),
    Rename(PathBuf, PathBuf),
    Config(PathBuf, Option<String>, Option<String>),
}

impl Change {
    fn apply(&self) -> Result<()> {
        match self {
            Self::Config(path, before, after) => {
                replace_config(path, before.as_deref(), after.as_deref())?
            }
            Self::Link(target, source) => symlink(source, target)?,
            Self::Unlink(target, source) => {
                ensure!(
                    matches_link(target, source),
                    "{} changed during installation",
                    target.display()
                );
                fs::remove_file(target)?;
            }
            Self::Rename(from, to) => {
                ensure!(!exists(to)?, "Refusing to overwrite {}", to.display());
                fs::rename(from, to)?;
            }
        }
        Ok(())
    }

    fn undo(&self) -> Result<()> {
        match self {
            Self::Config(path, before, after) => {
                Self::Config(path.clone(), after.clone(), before.clone()).apply()
            }
            Self::Link(target, source) => Self::Unlink(target.clone(), source.clone()).apply(),
            Self::Unlink(target, source) => Self::Link(target.clone(), source.clone()).apply(),
            Self::Rename(from, to) => Self::Rename(to.clone(), from.clone()).apply(),
        }
    }
}

fn transact(actions: &[Change], commit: impl FnOnce() -> Result<()>) -> Result<()> {
    let mut done = 0;
    let result = (|| {
        for action in actions {
            action.apply()?;
            done += 1;
        }
        commit()
    })();
    if let Err(error) = result {
        let mut failures = Vec::new();
        for action in actions[..done].iter().rev() {
            if let Err(error) = action.undo() {
                failures.push(format!("{error:#}"));
            }
        }
        if !failures.is_empty() {
            bail!(
                "{error:#}. Recovery also failed: {}. Preserve AGENTS-backup.md and installation state for manual recovery",
                failures.join("; ")
            );
        }
        return Err(error.context("No installation changes saved; prior changes rolled back"));
    }
    Ok(())
}

fn write_state(config: &Path, bytes: &[u8]) -> Result<()> {
    let temp = config.join(".north-install.lock/state.json");
    let result = (|| {
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&temp)?;
        file.write_all(bytes)?;
        file.sync_all()?;
        fs::rename(&temp, config.join(STATE))?;
        Ok(())
    })();
    if result.is_err() {
        let _ = fs::remove_file(temp);
    }
    result
}

fn read_config(path: &Path) -> Result<Option<String>> {
    if !exists(path)? {
        return Ok(None);
    }
    ensure!(
        fs::symlink_metadata(path)?.is_file(),
        "Expected a regular configuration file at {}; move the conflicting file or symlink aside first",
        path.display()
    );
    Ok(Some(
        fs::read_to_string(path).with_context(|| format!("Reading {}", path.display()))?,
    ))
}

fn replace_config(path: &Path, before: Option<&str>, after: Option<&str>) -> Result<()> {
    ensure!(
        read_config(path)?.as_deref() == before,
        "{} changed during installation",
        path.display()
    );
    let Some(after) = after else {
        if before.is_some() {
            fs::remove_file(path)?;
        }
        return Ok(());
    };
    let temp = path
        .parent()
        .unwrap()
        .join(".north-install.lock/config.json");
    let result = (|| {
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&temp)?;
        if before.is_some() {
            file.set_permissions(fs::metadata(path)?.permissions())?;
        }
        file.write_all(after.as_bytes())?;
        file.sync_all()?;
        fs::rename(&temp, path)?;
        Ok(())
    })();
    if result.is_err() {
        let _ = fs::remove_file(temp);
    }
    result
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    fn fixture() -> (TempDir, PathBuf, PathBuf) {
        let temp = tempfile::tempdir().unwrap();
        let repo = temp.path().join("repo");
        let config = temp.path().join("config");
        fs::create_dir_all(repo.join("assets/instructions")).unwrap();
        fs::create_dir_all(repo.join("assets/agents")).unwrap();
        fs::create_dir_all(repo.join("assets/commands")).unwrap();
        fs::write(repo.join("assets/instructions/core.md"), "north").unwrap();
        fs::write(
            repo.join("assets/opencode.json"),
            r#"{"plugin":["north-plugin"],"permission":{"edit":"ask"}}"#,
        )
        .unwrap();
        fs::write(repo.join("assets/agents/north-worker.md"), "agent").unwrap();
        fs::write(repo.join("assets/commands/north.md"), "command").unwrap();
        for name in ["one", "two"] {
            fs::create_dir_all(repo.join("assets/skills").join(name)).unwrap();
            fs::write(
                repo.join("assets/skills").join(name).join("SKILL.md"),
                "skill",
            )
            .unwrap();
        }
        (temp, repo, config)
    }

    fn add_commit_modes(repo: &Path) {
        for name in [COMMIT, AUTO_COMMIT] {
            let path = repo.join("assets/skills").join(name);
            fs::create_dir_all(&path).unwrap();
            fs::write(path.join("SKILL.md"), "skill").unwrap();
        }
    }

    #[test]
    fn merge_preserves_both_config_formats_and_original_instructions() {
        use std::os::unix::fs::PermissionsExt;
        let (_temp, repo, config) = fixture();
        fs::create_dir_all(&config).unwrap();
        fs::write(config.join("AGENTS.md"), "my instructions").unwrap();
        let json = r#"{"model":"custom","plugin":["my-plugin"]}"#;
        let jsonc = "{\n// settings\n\"permission\":{\"edit\":\"deny\",},\n}\n";
        fs::write(config.join("opencode.json"), json).unwrap();
        fs::write(config.join("opencode.jsonc"), jsonc).unwrap();
        fs::set_permissions(
            config.join("opencode.json"),
            fs::Permissions::from_mode(0o640),
        )
        .unwrap();
        let initial = Installation::load(&repo, &config).unwrap();
        initial.apply_with_merge(&BTreeSet::new(), true).unwrap();
        assert_eq!(
            fs::read_to_string(config.join("AGENTS.md")).unwrap(),
            "my instructions"
        );
        assert!(!config.join(BACKUP).exists());
        assert_eq!(
            fs::metadata(config.join(STATE))
                .unwrap()
                .permissions()
                .mode()
                & 0o777,
            0o600
        );
        assert_eq!(
            fs::metadata(config.join("opencode.json"))
                .unwrap()
                .permissions()
                .mode()
                & 0o777,
            0o640
        );
        let before = fs::read(config.join("opencode.json")).unwrap();
        let value: serde_json::Value = serde_json::from_slice(&before).unwrap();
        assert_eq!(value["model"], "custom");
        assert_eq!(
            value["plugin"],
            serde_json::json!(["my-plugin", "north-plugin"])
        );
        assert_eq!(
            value["instructions"],
            serde_json::json!([repo
                .canonicalize()
                .unwrap()
                .join("assets/instructions/core.md")])
        );
        let installed = Installation::load(&repo, &config).unwrap();
        assert!(installed.merging());
        installed.apply(&BTreeSet::new()).unwrap();
        assert_eq!(fs::read(config.join("opencode.json")).unwrap(), before);
        Installation::load(&repo, &config)
            .unwrap()
            .uninstall()
            .unwrap();
        assert_eq!(
            fs::read_to_string(config.join("opencode.json")).unwrap(),
            json
        );
        assert_eq!(
            fs::read_to_string(config.join("opencode.jsonc")).unwrap(),
            jsonc
        );
        assert_eq!(
            fs::read_to_string(config.join("AGENTS.md")).unwrap(),
            "my instructions"
        );
    }

    #[test]
    fn merge_modes_restore_instructions_when_switching_in_either_direction() {
        let (_temp, repo, config) = fixture();
        fs::create_dir_all(&config).unwrap();
        fs::write(config.join("AGENTS.md"), "original").unwrap();
        let initial = Installation::load(&repo, &config).unwrap();
        initial.apply(&BTreeSet::new()).unwrap();
        for _ in 0..2 {
            Installation::load(&repo, &config)
                .unwrap()
                .apply_with_merge(&BTreeSet::new(), true)
                .unwrap();
            assert_eq!(
                fs::read_to_string(config.join("AGENTS.md")).unwrap(),
                "original"
            );
            assert!(!config.join(BACKUP).exists());
            assert!(config.join("opencode.json").exists());
            Installation::load(&repo, &config)
                .unwrap()
                .apply_with_merge(&BTreeSet::new(), false)
                .unwrap();
            assert!(config.join("AGENTS.md").is_symlink());
            assert_eq!(fs::read_to_string(config.join(BACKUP)).unwrap(), "original");
            assert!(!config.join("opencode.json").exists());
        }
        Installation::load(&repo, &config)
            .unwrap()
            .uninstall()
            .unwrap();
        assert_eq!(
            fs::read_to_string(config.join("AGENTS.md")).unwrap(),
            "original"
        );
    }

    #[test]
    fn merge_updates_plugins_and_relocated_instructions_without_losing_user_edits() {
        let (temp, repo, config) = fixture();
        Installation::load(&repo, &config)
            .unwrap()
            .apply_with_merge(&BTreeSet::new(), true)
            .unwrap();
        let path = config.join("opencode.json");
        let mut edited: serde_json::Value =
            serde_json::from_slice(&fs::read(&path).unwrap()).unwrap();
        edited["plugin"]
            .as_array_mut()
            .unwrap()
            .push(serde_json::json!("later-plugin"));
        edited["model"] = serde_json::json!("user-model");
        fs::write(&path, serde_json::to_string_pretty(&edited).unwrap()).unwrap();
        let moved = temp.path().join("moved repo");
        fs::rename(repo, &moved).unwrap();
        fs::write(
            moved.join("assets/opencode.json"),
            r#"{"plugin":["north-v2"]}"#,
        )
        .unwrap();
        Installation::load(&moved, &config)
            .unwrap()
            .apply(&BTreeSet::new())
            .unwrap();
        let updated: serde_json::Value = serde_json::from_slice(&fs::read(&path).unwrap()).unwrap();
        assert_eq!(
            updated["plugin"],
            serde_json::json!(["later-plugin", "north-v2"])
        );
        assert_eq!(
            updated["instructions"],
            serde_json::json!([moved
                .canonicalize()
                .unwrap()
                .join("assets/instructions/core.md")])
        );
        Installation::load(&moved, &config)
            .unwrap()
            .uninstall()
            .unwrap();
        let remaining: serde_json::Value =
            serde_json::from_slice(&fs::read(path).unwrap()).unwrap();
        assert_eq!(remaining["plugin"], serde_json::json!(["later-plugin"]));
        assert_eq!(remaining["model"], "user-model");
    }

    #[test]
    fn merge_preflight_preserves_files_on_invalid_json_symlinks_and_link_conflicts() {
        for conflict in ["invalid", "symlink", "link"] {
            let (_temp, repo, config) = fixture();
            fs::create_dir_all(config.join("agents")).unwrap();
            fs::write(config.join("AGENTS.md"), "original").unwrap();
            let path = config.join("opencode.jsonc");
            match conflict {
                "invalid" => fs::write(&path, "{ broken").unwrap(),
                "symlink" => symlink(repo.join("assets/opencode.json"), &path).unwrap(),
                _ => {
                    fs::write(&path, "{}").unwrap();
                    fs::write(config.join("agents/north-worker.md"), "custom").unwrap();
                }
            }
            let before = fs::read(&path).unwrap();
            assert!(
                Installation::load(&repo, &config)
                    .unwrap()
                    .apply_with_merge(&BTreeSet::new(), true)
                    .is_err()
            );
            assert_eq!(fs::read(path).unwrap(), before);
            assert_eq!(
                fs::read_to_string(config.join("AGENTS.md")).unwrap(),
                "original"
            );
            assert!(!config.join(STATE).exists());
        }
    }

    #[test]
    fn failed_transaction_restores_configuration_and_rejects_concurrent_edits() {
        let temp = tempfile::tempdir().unwrap();
        fs::create_dir(temp.path().join(".north-install.lock")).unwrap();
        let path = temp.path().join("opencode.jsonc");
        let original = "{ /* original */ }\n";
        fs::write(&path, original).unwrap();
        assert!(
            transact(
                &[Change::Config(
                    path.clone(),
                    Some(original.into()),
                    Some("{}".into())
                )],
                || bail!("state write failed")
            )
            .is_err()
        );
        assert_eq!(fs::read_to_string(&path).unwrap(), original);
        assert!(
            Change::Config(path.clone(), Some("stale".into()), Some("{}".into()))
                .apply()
                .is_err()
        );
        assert_eq!(fs::read_to_string(path).unwrap(), original);
    }

    #[test]
    fn commit_modes_are_exclusive_and_survive_reruns() {
        let (_temp, repo, config) = fixture();
        add_commit_modes(&repo);
        let initial = Installation::load(&repo, &config).unwrap();
        assert!(initial.selected_skills().contains(AUTO_COMMIT));
        assert!(!initial.skills.iter().any(|name| name == COMMIT));
        initial.apply(&initial.selected_skills()).unwrap();
        assert!(matches_link(
            &config.join("skills/auto-commit"),
            &repo
                .canonicalize()
                .unwrap()
                .join("assets/skills/auto-commit")
        ));
        assert!(!exists(&config.join("skills/commit")).unwrap());

        let installed = Installation::load(&repo, &config).unwrap();
        assert!(installed.selected_skills().contains(AUTO_COMMIT));
        installed.apply(&BTreeSet::new()).unwrap();
        let manual = Installation::load(&repo, &config).unwrap();
        assert!(manual.selected_skills().is_empty());
        assert_eq!(
            manual.resolved_skills(&BTreeSet::new()).unwrap(),
            BTreeSet::from([COMMIT.into()])
        );
        assert!(matches_link(
            &config.join("skills/commit"),
            &repo.canonicalize().unwrap().join("assets/skills/commit")
        ));
        assert!(!exists(&config.join("skills/auto-commit")).unwrap());
        manual.apply(&manual.selected_skills()).unwrap();
        let manual = Installation::load(&repo, &config).unwrap();
        manual.apply(&BTreeSet::from([AUTO_COMMIT.into()])).unwrap();
        assert!(!exists(&config.join("skills/commit")).unwrap());
        let automatic = Installation::load(&repo, &config).unwrap();
        assert_eq!(
            automatic.selected_skills(),
            BTreeSet::from([AUTO_COMMIT.into()])
        );
        let state_before = fs::read(config.join(STATE)).unwrap();
        assert!(
            automatic
                .apply(&BTreeSet::from([COMMIT.into(), AUTO_COMMIT.into()]))
                .is_err()
        );
        assert_eq!(fs::read(config.join(STATE)).unwrap(), state_before);
        automatic.uninstall().unwrap();
        assert!(!exists(&config.join("skills/auto-commit")).unwrap());
        assert!(!exists(&config.join("skills/commit")).unwrap());
    }

    #[test]
    fn upgrading_adds_manual_commit_without_enabling_auto_commit() {
        let (_temp, repo, config) = fixture();
        Installation::load(&repo, &config)
            .unwrap()
            .apply(&BTreeSet::from(["two".into()]))
            .unwrap();
        add_commit_modes(&repo);
        let upgraded = Installation::load(&repo, &config).unwrap();
        assert_eq!(upgraded.selected_skills(), BTreeSet::from(["two".into()]));
        upgraded.apply(&upgraded.selected_skills()).unwrap();
        assert!(config.join("skills/commit").is_symlink());
        assert!(!exists(&config.join("skills/auto-commit")).unwrap());
        assert!(config.join("skills/two").is_symlink());
    }

    #[test]
    fn commit_mode_conflicts_preserve_links_and_state() {
        for (active, next) in [(COMMIT, AUTO_COMMIT), (AUTO_COMMIT, COMMIT)] {
            let (_temp, repo, config) = fixture();
            add_commit_modes(&repo);
            Installation::load(&repo, &config)
                .unwrap()
                .apply(&BTreeSet::from([active.into()]))
                .unwrap();
            let state_before = fs::read(config.join(STATE)).unwrap();
            // A replacement of the old mode cannot be silently left active.
            fs::remove_file(config.join("skills").join(active)).unwrap();
            fs::create_dir(config.join("skills").join(active)).unwrap();
            let custom = config.join("skills").join(active).join("SKILL.md");
            fs::write(&custom, "custom instructions").unwrap();
            assert!(
                Installation::load(&repo, &config)
                    .unwrap()
                    .apply(&BTreeSet::from([next.into()]))
                    .is_err()
            );
            assert_eq!(fs::read_to_string(custom).unwrap(), "custom instructions");
            assert!(!exists(&config.join("skills").join(next)).unwrap());
            assert_eq!(fs::read(config.join(STATE)).unwrap(), state_before);
        }
    }

    #[test]
    fn upgrade_adds_commands_even_with_no_skills_selected() {
        let (_temp, repo, config) = fixture();
        let command = repo.join("assets/commands/north.md");
        fs::remove_file(&command).unwrap();
        Installation::load(&repo, &config)
            .unwrap()
            .apply(&BTreeSet::new())
            .unwrap();
        fs::write(&command, "command").unwrap();
        Installation::load(&repo, &config)
            .unwrap()
            .apply(&BTreeSet::new())
            .unwrap();
        assert!(matches_link(
            &config.join("commands/north.md"),
            &command.canonicalize().unwrap()
        ));
        // Tracked commands can still be removed after their source is deleted.
        fs::remove_file(command).unwrap();
        Installation::load(&repo, &config)
            .unwrap()
            .uninstall()
            .unwrap();
        assert!(!exists(&config.join("commands")).unwrap());
    }

    #[test]
    fn selections_follow_links_and_new_skills_start_disabled_on_rerun() {
        let (_temp, repo, config) = fixture();
        let initial = Installation::load(&repo, &config).unwrap();
        assert_eq!(
            initial.selected_skills(),
            BTreeSet::from(["one".into(), "two".into()])
        );
        initial.apply(&BTreeSet::from(["two".into()])).unwrap();
        fs::create_dir_all(repo.join("assets/skills/three")).unwrap();
        fs::write(repo.join("assets/skills/three/SKILL.md"), "new").unwrap();
        let updated = Installation::load(&repo, &config).unwrap();
        assert_eq!(updated.selected_skills(), BTreeSet::from(["two".into()]));
        fs::remove_file(config.join("skills/two")).unwrap();
        assert!(
            Installation::load(&repo, &config)
                .unwrap()
                .selected_skills()
                .is_empty()
        );
    }

    #[test]
    fn uninstall_removes_tracked_assets_deleted_from_checkout() {
        let (_temp, repo, config) = fixture();
        let initial = Installation::load(&repo, &config).unwrap();
        initial.apply(&initial.skill_names()).unwrap();
        fs::remove_dir_all(repo.join("assets/skills/two")).unwrap();
        Installation::load(&repo, &config)
            .unwrap()
            .uninstall()
            .unwrap();
        assert!(!exists(&config.join("skills/two")).unwrap());
        assert!(!exists(&config.join(STATE)).unwrap());
    }

    #[test]
    fn failed_commit_restores_original_file_and_removed_link() {
        let temp = tempfile::tempdir().unwrap();
        let original = temp.path().join("AGENTS.md");
        let backup = temp.path().join(BACKUP);
        let skill = temp.path().join("skill");
        let source = temp.path().join("missing-source");
        fs::write(&original, b"original\0bytes\n").unwrap();
        symlink(&source, &skill).unwrap();
        let result = transact(
            &[
                Change::Rename(original.clone(), backup.clone()),
                Change::Link(original.clone(), source.clone()),
                Change::Unlink(skill.clone(), source.clone()),
            ],
            || bail!("simulated state write failure"),
        );
        assert!(result.is_err());
        assert_eq!(fs::read(&original).unwrap(), b"original\0bytes\n");
        assert!(!exists(&backup).unwrap());
        assert!(matches_link(&skill, &source));
    }

    #[test]
    fn failed_link_rolls_back_backup_without_overwriting_conflict() {
        let temp = tempfile::tempdir().unwrap();
        let original = temp.path().join("AGENTS.md");
        let backup = temp.path().join(BACKUP);
        let conflict = temp.path().join("conflict");
        fs::write(&original, "original").unwrap();
        fs::write(&conflict, "keep").unwrap();
        assert!(
            transact(
                &[
                    Change::Rename(original.clone(), backup.clone()),
                    Change::Link(conflict.clone(), original.clone()),
                ],
                || Ok(())
            )
            .is_err()
        );
        assert_eq!(fs::read_to_string(original).unwrap(), "original");
        assert_eq!(fs::read_to_string(conflict).unwrap(), "keep");
        assert!(!exists(&backup).unwrap());
    }

    #[test]
    fn stale_menu_and_parallel_installer_cannot_overwrite_state() {
        let (_temp, repo, config) = fixture();
        let stale = Installation::load(&repo, &config).unwrap();
        let active = Installation::load(&repo, &config).unwrap();
        active.apply(&BTreeSet::new()).unwrap();
        assert!(stale.apply(&stale.skill_names()).is_err());
        let current = Installation::load(&repo, &config).unwrap();
        fs::create_dir(config.join(".north-install.lock")).unwrap();
        assert!(current.uninstall().is_err());
        assert!(exists(&config.join("AGENTS.md")).unwrap());
    }

    #[test]
    fn invalid_state_cannot_remove_paths_outside_config() {
        let (_temp, repo, config) = fixture();
        let initial = Installation::load(&repo, &config).unwrap();
        initial.apply(&initial.skill_names()).unwrap();
        let mut state = read_state(&config).unwrap().unwrap();
        state.links.insert(
            PathBuf::from("skills/../../outside"),
            repo.join("assets/skills/two"),
        );
        fs::write(config.join(STATE), serde_json::to_vec(&state).unwrap()).unwrap();
        assert!(Installation::load(&repo, &config).is_err());
    }
}
