use crate::config::ConfigMerge;
use crate::tool::Tool;
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
const COMMIT: &str = "commit";
pub const AUTO_COMMIT: &str = "auto-commit";
pub const NORTH_PIPELINE: &str = "north-pipeline";
pub const PIPELINE_SKILLS: &[&str] = &[
    "north-plan",
    "north-explore",
    "north-execute",
    "north-save",
    "invoke-memory",
];

pub fn pipeline_enabled(selected: &BTreeSet<String>) -> bool {
    selected.contains(NORTH_PIPELINE) || PIPELINE_SKILLS.iter().any(|name| selected.contains(*name))
}

fn skill_dependencies(name: &str) -> &'static [&'static str] {
    match name {
        "north-plan" => &[
            "north-sources",
            "clarify-requirements",
            "north-explore",
            "invoke-memory",
        ],
        "north-explore" => &["research"],
        "north-execute" => &["north-sources", "invoke-memory", "subagent-usage"],
        "north-save"
        | "invoke-memory"
        | "subagent-usage"
        | "research"
        | "clarify-requirements"
        | "handoff"
        | "dry-skillify"
        | "domain-modeling"
        | "prototype"
        | "skill-evaluation" => &["north-sources"],
        _ => &[],
    }
}

// Skills are shared; instructions, agents, and commands are tool-specific.
// Earlier OpenCode installations linked to the flat `assets/` layout.
fn asset_sources(tool: Tool, repo: &Path, relative: &Path) -> Vec<PathBuf> {
    let assets = repo.join("assets");
    let mut sources = Vec::new();
    if relative == Path::new(tool.instructions()) {
        sources.push(assets.join(tool.id()).join("instructions/core.md"));
        if tool == Tool::OpenCode {
            sources.push(assets.join("instructions/core.md"));
        }
    } else if relative.starts_with("skills") {
        sources.push(assets.join(relative));
    } else {
        sources.push(assets.join(tool.id()).join(relative));
        if tool == Tool::OpenCode {
            sources.push(assets.join(relative));
        }
    }
    sources
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
struct State {
    version: u32,
    #[serde(default)]
    tool: Tool,
    repo: PathBuf,
    backup: bool,
    links: BTreeMap<PathBuf, PathBuf>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    configs: Vec<ConfigMerge>,
}

pub struct Installation {
    pub tool: Tool,
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

fn read_state(config: &Path, tool: Tool) -> Result<Option<State>> {
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
    ensure!(
        state.tool == tool,
        "North installation state at {} belongs to {}",
        path.display(),
        state.tool.label()
    );
    for (relative, source) in &state.links {
        let parts: Vec<_> = relative.components().collect();
        ensure!(
            parts
                .iter()
                .all(|part| matches!(part, Component::Normal(_))),
            "Invalid path in North installation state"
        );
        if relative != Path::new(tool.instructions()) {
            ensure!(
                parts.len() == 2
                    && ["skills", "agents", "commands"]
                        .iter()
                        .any(|folder| parts[0].as_os_str() == *folder),
                "Invalid link in North installation state"
            );
        }
        ensure!(
            asset_sources(tool, &state.repo, relative).contains(source),
            "Invalid source in North installation state"
        );
    }
    let mut names = BTreeSet::new();
    for config in &state.configs {
        ensure!(
            tool.config_files().contains(&config.file.as_str()) && names.insert(&config.file),
            "Invalid configuration file in North installation state"
        );
    }
    ensure!(
        state.links.contains_key(Path::new(tool.instructions())) || !state.configs.is_empty(),
        "Missing instructions link in North installation state"
    );
    Ok(Some(state))
}

impl Installation {
    pub fn load(repo: &Path, tool: Tool, config: &Path) -> Result<Self> {
        let repo = repo.canonicalize().context("Locating North checkout")?;
        check_directory(config)?;
        check_directory(&config.join("agents"))?;
        check_directory(&config.join("skills"))?;
        check_directory(&config.join("commands"))?;
        let mut available = BTreeMap::new();
        let instructions = PathBuf::from(tool.instructions());
        let core = asset_sources(tool, &repo, &instructions).remove(0);
        ensure!(core.is_file(), "Missing {}", core.display());
        available.insert(instructions, core);
        let mut skills = Vec::new();
        for folder in ["agents", "skills", "commands"] {
            let directory = if folder == "skills" {
                repo.join("assets/skills")
            } else {
                repo.join("assets").join(tool.id()).join(folder)
            };
            for entry in fs::read_dir(&directory)
                .with_context(|| format!("Reading {}", directory.display()))?
            {
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
                    if folder == "skills"
                        && name != COMMIT
                        && !PIPELINE_SKILLS.contains(&name.as_str())
                    {
                        skills.push(name.clone());
                    }
                    available.insert(Path::new(folder).join(name), path);
                }
            }
        }
        if PIPELINE_SKILLS
            .iter()
            .any(|name| available.contains_key(&Path::new("skills").join(name)))
        {
            skills.push(NORTH_PIPELINE.into());
        }
        skills.sort();
        ensure!(
            available.contains_key(Path::new("skills/commit"))
                == available.contains_key(Path::new("skills/auto-commit")),
            "The commit and auto-commit skills must be bundled together"
        );
        Ok(Self {
            state: read_state(config, tool)?,
            tool,
            repo,
            config: config.to_owned(),
            available,
            skills,
        })
    }

    pub fn skill_names(&self) -> BTreeSet<String> {
        self.skills.iter().cloned().collect()
    }

    // Adopt links from the previous shell installer, which had no state file,
    // including links into the earlier flat asset layout.
    fn owned_links(&self) -> BTreeMap<PathBuf, PathBuf> {
        self.state
            .as_ref()
            .map(|state| state.links.clone())
            .unwrap_or_else(|| {
                self.available
                    .keys()
                    .filter_map(|relative| {
                        let target = self.config.join(relative);
                        asset_sources(self.tool, &self.repo, relative)
                            .into_iter()
                            .find(|source| matches_link(&target, source))
                            .map(|source| (relative.clone(), source))
                    })
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
        let mut selected: BTreeSet<_> = self
            .skills
            .iter()
            .filter(|name| {
                let relative = Path::new("skills").join(name);
                owned
                    .get(&relative)
                    .is_some_and(|source| matches_link(&self.config.join(relative), source))
            })
            .cloned()
            .collect();
        if PIPELINE_SKILLS.iter().any(|name| {
            let relative = Path::new("skills").join(name);
            owned
                .get(&relative)
                .is_some_and(|source| matches_link(&self.config.join(relative), source))
        }) {
            selected.insert(NORTH_PIPELINE.into());
        }
        selected
    }

    fn lock(&self) -> Result<Lock> {
        check_directory(&self.config)?;
        fs::create_dir_all(&self.config)?;
        let path = self.config.join(".north-install.lock");
        fs::create_dir(&path).with_context(|| format!("Cannot lock installation. Another installer may be running; if it crashed, remove {} and retry", path.display()))?;
        let lock = Lock(path);
        ensure!(
            read_state(&self.config, self.tool)? == self.state,
            "Installation changed while the menu was open; rerun install.sh"
        );
        check_directory(&self.config.join("agents"))?;
        check_directory(&self.config.join("skills"))?;
        check_directory(&self.config.join("commands"))?;
        Ok(lock)
    }

    // Checkbox options expand into concrete skill assets before any mutation.
    pub fn resolved_skills(&self, selected: &BTreeSet<String>) -> Result<BTreeSet<String>> {
        let mut known = self.skill_names();
        // Accept previous CLI skill names as aliases for the complete pipeline.
        known.insert(NORTH_PIPELINE.into());
        known.extend(PIPELINE_SKILLS.iter().map(|name| (*name).to_owned()));
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
        resolved.remove(NORTH_PIPELINE);
        if pipeline_enabled(selected) {
            for &name in PIPELINE_SKILLS {
                ensure!(
                    self.available.contains_key(&Path::new("skills").join(name)),
                    "North pipeline requires missing bundled skill {name}; restore the North checkout before installing"
                );
                resolved.insert(name.into());
            }
            for &name in self.tool.pipeline_commands() {
                ensure!(
                    self.available
                        .contains_key(&Path::new("commands").join(name)),
                    "North pipeline requires missing bundled command {name}; restore the North checkout before installing"
                );
            }
        }
        if known.contains(COMMIT) && !selected.contains(AUTO_COMMIT) {
            resolved.insert(COMMIT.into());
        }
        let mut pending: Vec<_> = resolved.iter().cloned().collect();
        while let Some(name) = pending.pop() {
            for &dependency in skill_dependencies(&name) {
                ensure!(
                    self.available
                        .contains_key(&Path::new("skills").join(dependency)),
                    "Skill {name} requires missing bundled skill {dependency}; restore the North checkout before installing"
                );
                if resolved.insert(dependency.into()) {
                    pending.push(dependency.into());
                }
            }
        }
        Ok(resolved)
    }

    #[cfg(test)]
    pub fn apply(&self, selected: &BTreeSet<String>) -> Result<()> {
        self.apply_with_merge(selected, self.merging())
    }

    pub fn apply_with_merge(&self, selected: &BTreeSet<String>, merge: bool) -> Result<()> {
        let pipeline = pipeline_enabled(selected);
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
        let instructions = Path::new(self.tool.instructions());
        let pipeline_commands = self.tool.pipeline_commands();
        let desired: BTreeMap<_, _> = self
            .available
            .iter()
            .filter(|(relative, _)| {
                (!merge || relative.as_path() != instructions)
                    && (pipeline
                        || !relative.starts_with("commands")
                        || !pipeline_commands
                            .contains(&relative.file_name().unwrap().to_str().unwrap()))
                    && (!relative.starts_with("skills")
                        || selected.contains(relative.file_name().unwrap().to_str().unwrap()))
            })
            .map(|(relative, source)| (relative.clone(), source.clone()))
            .collect();
        let agents = self.config.join(instructions);
        let backup = self.config.join(self.tool.backup());
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
        // When merge mode edits the instructions file itself, it edits the
        // original instructions restored from the backup, not North's link.
        let linked_instructions = owned
            .get(instructions)
            .is_some_and(|source| matches_link(&agents, source));
        let instructions_before = if merge && linked_instructions {
            Some(if has_backup {
                read_config(&backup)?
            } else {
                None
            })
        } else {
            None
        };
        let (configs, config_actions) = self.config_changes(merge, instructions_before)?;
        let instructions_removed = config_actions
            .iter()
            .any(|action| matches!(action, Change::Config(path, _, None) if *path == agents));
        // Removing additions precedes relinking the instructions; adding them
        // follows restoring the original instructions from the backup.
        let (mut actions, mut config_actions) = if merge {
            (Vec::new(), config_actions)
        } else {
            (config_actions, Vec::new())
        };
        for (relative, source) in &desired {
            let target = self.config.join(relative);
            if matches_link(&target, source) {
                continue;
            }
            let removed = relative == instructions && instructions_removed;
            if exists(&target)? && !removed {
                if owned
                    .get(relative)
                    .is_some_and(|old| matches_link(&target, old))
                {
                    actions.push(Change::Unlink(target.clone(), owned[relative].clone()));
                } else if relative == instructions && (self.state.is_none() || self.merging()) {
                    ensure!(
                        !fs::symlink_metadata(&agents)?.is_dir(),
                        "{} must be a file or symlink",
                        instructions.display()
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
                linked_instructions || !exists(&agents)?,
                "{} was changed outside North; move it aside before restoring the backup",
                instructions.display()
            );
            actions.push(Change::Rename(backup, agents));
            has_backup = false;
        }
        actions.append(&mut config_actions);
        let state = State {
            version: if configs.is_empty() { 1 } else { 2 },
            tool: self.tool,
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

    fn merge_config(&self, name: String, original: Option<String>) -> Result<ConfigMerge> {
        let core = self.available[Path::new(self.tool.instructions())].clone();
        match self.tool {
            Tool::OpenCode => {
                let path = self.repo.join("assets/opencode/opencode.json");
                let mut north = serde_json::from_slice::<serde_json::Value>(
                    &fs::read(&path).with_context(|| format!("Reading {}", path.display()))?,
                )?;
                ensure!(north.is_object(), "North configuration must be an object");
                let instructions = north
                    .as_object_mut()
                    .unwrap()
                    .entry("instructions")
                    .or_insert(serde_json::json!([]));
                let instructions = instructions
                    .as_array_mut()
                    .context("North instructions must be an array")?;
                let core = serde_json::json!(core);
                if !instructions.contains(&core) {
                    instructions.push(core);
                }
                ConfigMerge::new(name, original, &north)
            }
            Tool::Claude => {
                let core = core
                    .to_str()
                    .context("The North checkout path must be UTF-8")?;
                ConfigMerge::import(name, original, &format!("@{core}"))
            }
        }
    }

    fn config_changes(
        &self,
        merge: bool,
        instructions_before: Option<Option<String>>,
    ) -> Result<(Vec<ConfigMerge>, Vec<Change>)> {
        let previous = self
            .state
            .as_ref()
            .map(|state| state.configs.as_slice())
            .unwrap_or_default();
        if !merge && previous.is_empty() {
            return Ok((Vec::new(), Vec::new()));
        }
        let mut names: BTreeSet<String> =
            previous.iter().map(|config| config.file.clone()).collect();
        if merge {
            for name in self.tool.config_files() {
                if exists(&self.config.join(name))? {
                    names.insert((*name).into());
                }
            }
            if names.is_empty() {
                names.insert(self.tool.config_files()[0].into());
            }
        }
        let mut configs = Vec::new();
        let mut actions = Vec::new();
        for name in names {
            let path = self.config.join(&name);
            let before = match &instructions_before {
                Some(before) if name == self.tool.instructions() => before.clone(),
                _ => read_config(&path)?,
            };
            let original = match previous.iter().find(|config| config.file == name) {
                Some(config) => config
                    .remove(before.as_deref())
                    .with_context(|| format!("Updating {}", path.display()))?,
                None => before.clone(),
            };
            let after = if merge {
                let config = self
                    .merge_config(name, original)
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
        let (_, mut actions) = self.config_changes(false, None)?;
        for (relative, source) in &owned {
            let target = self.config.join(relative);
            if matches_link(&target, source) {
                actions.push(Change::Unlink(target, source.clone()));
            }
        }
        if self.state.as_ref().is_some_and(|state| state.backup) {
            let instructions = Path::new(self.tool.instructions());
            let agents = self.config.join(instructions);
            let backup = self.config.join(self.tool.backup());
            ensure!(
                exists(&backup)? && !fs::symlink_metadata(&backup)?.is_dir(),
                "Saved {} is missing or invalid; restore it before uninstalling",
                backup.display()
            );
            ensure!(
                !exists(&agents)?
                    || owned
                        .get(instructions)
                        .is_some_and(|source| matches_link(&agents, source)),
                "{} was changed outside North; move it aside before restoring the backup",
                instructions.display()
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
                "{error:#}. Recovery also failed: {}. Preserve the instructions backup and installation state for manual recovery",
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

    const BACKUP: &str = "AGENTS-backup.md";
    const PIPELINE_COMMANDS: &[&str] = &["north-plan.md", "north-execute.md", "north-save.md"];

    fn fixture() -> (TempDir, PathBuf, PathBuf) {
        let temp = tempfile::tempdir().unwrap();
        let repo = temp.path().join("repo");
        let config = temp.path().join("config");
        fs::create_dir_all(repo.join("assets/opencode/instructions")).unwrap();
        fs::create_dir_all(repo.join("assets/opencode/agents")).unwrap();
        fs::create_dir_all(repo.join("assets/opencode/commands")).unwrap();
        fs::write(repo.join("assets/opencode/instructions/core.md"), "north").unwrap();
        fs::write(
            repo.join("assets/opencode/opencode.json"),
            r#"{"plugin":["north-plugin"],"permission":{"edit":"ask"}}"#,
        )
        .unwrap();
        fs::write(repo.join("assets/opencode/agents/north-worker.md"), "agent").unwrap();
        fs::write(repo.join("assets/opencode/commands/north.md"), "command").unwrap();
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

    fn add_claude(repo: &Path) {
        fs::create_dir_all(repo.join("assets/claude/instructions")).unwrap();
        fs::create_dir_all(repo.join("assets/claude/agents")).unwrap();
        fs::create_dir_all(repo.join("assets/claude/commands")).unwrap();
        fs::write(
            repo.join("assets/claude/instructions/core.md"),
            "claude north",
        )
        .unwrap();
        fs::write(
            repo.join("assets/claude/agents/north-worker.md"),
            "claude agent",
        )
        .unwrap();
        fs::write(
            repo.join("assets/claude/commands/north.md"),
            "claude command",
        )
        .unwrap();
    }

    fn add_commit_modes(repo: &Path) {
        for name in [COMMIT, AUTO_COMMIT] {
            let path = repo.join("assets/skills").join(name);
            fs::create_dir_all(&path).unwrap();
            fs::write(path.join("SKILL.md"), "skill").unwrap();
        }
    }

    fn add_pipeline(repo: &Path) {
        for name in [
            "north-plan",
            "north-explore",
            "north-execute",
            "north-save",
            "invoke-memory",
            "north-sources",
            "clarify-requirements",
            "research",
            "subagent-usage",
        ] {
            let path = repo.join("assets/skills").join(name);
            fs::create_dir_all(&path).unwrap();
            fs::write(path.join("SKILL.md"), "skill").unwrap();
        }
        for name in ["north-plan", "north-execute", "north-save"] {
            fs::write(
                repo.join("assets/opencode/commands")
                    .join(format!("{name}.md")),
                "command",
            )
            .unwrap();
        }
    }

    #[test]
    fn shared_plan_contract_is_available_without_enabling_pipeline() {
        let repo = Path::new(env!("CARGO_MANIFEST_DIR")).parent().unwrap();
        let temp = tempfile::tempdir().unwrap();
        let config = temp.path().join("config");
        let contract = "skills/north-sources/references/plan-format.md";
        let expected = fs::read_to_string(repo.join("assets").join(contract)).unwrap();
        for name in [
            "subagent-usage",
            "handoff",
            "research",
            "clarify-requirements",
            "dry-skillify",
            "domain-modeling",
            "prototype",
            "skill-evaluation",
        ] {
            let installation = Installation::load(repo, Tool::OpenCode, &config).unwrap();
            installation.apply(&BTreeSet::from([name.into()])).unwrap();
            assert!(config.join("skills").join(name).is_symlink());
            assert_eq!(fs::read_to_string(config.join(contract)).unwrap(), expected);
            assert!(!config.join("skills/north-plan").exists());
            for command in PIPELINE_COMMANDS {
                assert!(!config.join("commands").join(command).exists());
            }
        }
    }

    #[test]
    fn pipeline_group_and_legacy_names_resolve_complete_pipeline() {
        let (_temp, repo, config) = fixture();
        add_pipeline(&repo);
        add_commit_modes(&repo);
        let installation = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        assert!(installation.skill_names().contains(NORTH_PIPELINE));
        assert!(installation.selected_skills().contains(NORTH_PIPELINE));
        assert!(
            PIPELINE_SKILLS
                .iter()
                .all(|name| !installation.skill_names().contains(*name))
        );
        let expected: BTreeSet<String> = [
            COMMIT,
            "north-plan",
            "north-explore",
            "north-execute",
            "north-save",
            "invoke-memory",
            "north-sources",
            "clarify-requirements",
            "research",
            "subagent-usage",
        ]
        .into_iter()
        .map(String::from)
        .collect();
        for &name in [NORTH_PIPELINE].iter().chain(PIPELINE_SKILLS) {
            let selected = BTreeSet::from([name.into()]);
            assert!(pipeline_enabled(&selected));
            assert_eq!(installation.resolved_skills(&selected).unwrap(), expected);
        }
        let mut automatic = expected.clone();
        automatic.remove(COMMIT);
        automatic.insert(AUTO_COMMIT.into());
        assert_eq!(
            installation
                .resolved_skills(&BTreeSet::from([
                    NORTH_PIPELINE.into(),
                    "north-plan".into(),
                    AUTO_COMMIT.into(),
                ]))
                .unwrap(),
            automatic
        );
        assert_eq!(
            installation
                .resolved_skills(&BTreeSet::from(["research".into()]))
                .unwrap(),
            BTreeSet::from([COMMIT.into(), "research".into(), "north-sources".into()])
        );
    }

    #[test]
    fn pipeline_toggle_controls_its_skills_and_commands_together() {
        let (_temp, repo, config) = fixture();
        add_pipeline(&repo);
        let selected = BTreeSet::from([NORTH_PIPELINE.into()]);
        let initial = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        let resolved = initial.resolved_skills(&selected).unwrap();
        initial.apply(&selected).unwrap();
        for name in &resolved {
            assert!(matches_link(
                &config.join("skills").join(name),
                &repo
                    .canonicalize()
                    .unwrap()
                    .join("assets/skills")
                    .join(name)
            ));
        }
        assert!(!exists(&config.join("skills/one")).unwrap());
        let installed = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        assert_eq!(
            installed.selected_skills(),
            BTreeSet::from([
                NORTH_PIPELINE.into(),
                "north-sources".into(),
                "clarify-requirements".into(),
                "research".into(),
                "subagent-usage".into(),
            ])
        );
        let state_before = fs::read(config.join(STATE)).unwrap();
        installed.apply(&installed.selected_skills()).unwrap();
        assert_eq!(fs::read(config.join(STATE)).unwrap(), state_before);

        let installed = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        let mut disabled = installed.selected_skills();
        disabled.remove(NORTH_PIPELINE);
        installed.apply(&disabled).unwrap();
        for name in PIPELINE_SKILLS {
            assert!(!exists(&config.join("skills").join(name)).unwrap());
        }
        for name in PIPELINE_COMMANDS {
            assert!(!exists(&config.join("commands").join(name)).unwrap());
        }
        assert!(config.join("commands/north.md").is_symlink());
        for name in &disabled {
            assert!(config.join("skills").join(name).is_symlink());
        }
        let installed = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        assert!(!pipeline_enabled(&installed.selected_skills()));
        disabled.insert(NORTH_PIPELINE.into());
        installed.apply(&disabled).unwrap();
        assert_eq!(fs::read(config.join(STATE)).unwrap(), state_before);
        for name in PIPELINE_COMMANDS {
            assert!(config.join("commands").join(name).is_symlink());
        }
        Installation::load(&repo, Tool::OpenCode, &config)
            .unwrap()
            .uninstall()
            .unwrap();
        assert!(!exists(&config.join("skills/north-save")).unwrap());
        assert!(!exists(&config.join("commands")).unwrap());
    }

    #[test]
    fn upgrading_keeps_pipeline_skills_and_commands_disabled() {
        let (_temp, repo, config) = fixture();
        let selected = BTreeSet::from(["two".into()]);
        Installation::load(&repo, Tool::OpenCode, &config)
            .unwrap()
            .apply(&selected)
            .unwrap();
        add_pipeline(&repo);
        let upgraded = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        assert_eq!(upgraded.selected_skills(), selected);
        upgraded.apply(&upgraded.selected_skills()).unwrap();
        for name in ["north-plan", "north-execute", "north-save"] {
            let relative = Path::new("commands").join(format!("{name}.md"));
            assert!(!exists(&config.join(&relative)).unwrap());
            assert!(!exists(&config.join("skills").join(name)).unwrap());
        }
        assert!(!exists(&config.join("skills/research")).unwrap());
    }

    #[test]
    fn existing_partial_pipeline_installations_migrate_to_the_group() {
        for name in PIPELINE_SKILLS {
            let (_temp, repo, config) = fixture();
            Installation::load(&repo, Tool::OpenCode, &config)
                .unwrap()
                .apply(&BTreeSet::new())
                .unwrap();
            add_pipeline(&repo);
            let mut state = read_state(&config, Tool::OpenCode).unwrap().unwrap();
            let relative = Path::new("skills").join(name);
            let source = repo.canonicalize().unwrap().join("assets").join(&relative);
            symlink(&source, config.join(&relative)).unwrap();
            state.links.insert(relative, source);
            fs::write(
                config.join(STATE),
                serde_json::to_vec_pretty(&state).unwrap(),
            )
            .unwrap();
            let upgraded = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
            assert_eq!(
                upgraded.selected_skills(),
                BTreeSet::from([NORTH_PIPELINE.into()])
            );
            upgraded.apply(&upgraded.selected_skills()).unwrap();
            for skill in PIPELINE_SKILLS {
                assert!(config.join("skills").join(skill).is_symlink());
            }
            for command in PIPELINE_COMMANDS {
                assert!(config.join("commands").join(command).is_symlink());
            }
        }
    }

    #[test]
    fn disabling_pipeline_preserves_replacements_and_reenable_fails_atomically() {
        let (_temp, repo, config) = fixture();
        add_pipeline(&repo);
        let selected = BTreeSet::from([NORTH_PIPELINE.into()]);
        Installation::load(&repo, Tool::OpenCode, &config)
            .unwrap()
            .apply(&selected)
            .unwrap();
        let command = config.join("commands/north-execute.md");
        fs::remove_file(&command).unwrap();
        fs::write(&command, "custom command").unwrap();
        let memory = config.join("skills/invoke-memory");
        fs::remove_file(&memory).unwrap();
        fs::create_dir(&memory).unwrap();
        fs::write(memory.join("SKILL.md"), "custom memory").unwrap();
        Installation::load(&repo, Tool::OpenCode, &config)
            .unwrap()
            .apply(&BTreeSet::new())
            .unwrap();
        assert_eq!(fs::read_to_string(&command).unwrap(), "custom command");
        assert_eq!(
            fs::read_to_string(memory.join("SKILL.md")).unwrap(),
            "custom memory"
        );
        assert!(!exists(&config.join("commands/north-plan.md")).unwrap());
        assert!(!exists(&config.join("skills/north-plan")).unwrap());
        let disabled = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        assert!(!pipeline_enabled(&disabled.selected_skills()));
        let state_before = fs::read(config.join(STATE)).unwrap();
        assert!(disabled.apply(&selected).is_err());
        assert_eq!(fs::read(config.join(STATE)).unwrap(), state_before);
        assert!(!exists(&config.join("commands/north-plan.md")).unwrap());
        assert!(!exists(&config.join("skills/north-plan")).unwrap());
        disabled.uninstall().unwrap();
        assert_eq!(fs::read_to_string(command).unwrap(), "custom command");
        assert_eq!(
            fs::read_to_string(memory.join("SKILL.md")).unwrap(),
            "custom memory"
        );
    }

    #[test]
    fn missing_pipeline_assets_fail_preflight_but_do_not_prevent_uninstall() {
        for relative in [
            "skills/invoke-memory/SKILL.md",
            "opencode/commands/north-save.md",
        ] {
            let (_temp, repo, config) = fixture();
            add_pipeline(&repo);
            let selected = BTreeSet::from([NORTH_PIPELINE.into()]);
            Installation::load(&repo, Tool::OpenCode, &config)
                .unwrap()
                .apply(&selected)
                .unwrap();
            let state_before = fs::read(config.join(STATE)).unwrap();
            fs::remove_file(repo.join("assets").join(relative)).unwrap();
            let installed = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
            assert!(
                installed
                    .apply(&selected)
                    .unwrap_err()
                    .to_string()
                    .contains("North pipeline requires missing bundled")
            );
            assert_eq!(fs::read(config.join(STATE)).unwrap(), state_before);
            assert!(config.join("skills/north-plan").is_symlink());
            assert!(!config.join(".north-install.lock").exists());
            installed.uninstall().unwrap();
            assert!(!exists(&config.join("commands")).unwrap());
            assert!(!exists(&config.join("skills")).unwrap());
        }
    }

    #[test]
    fn missing_transitive_dependency_fails_before_installation_changes() {
        let (_temp, repo, config) = fixture();
        add_pipeline(&repo);
        let selected = BTreeSet::from(["north-plan".into()]);
        Installation::load(&repo, Tool::OpenCode, &config)
            .unwrap()
            .apply(&selected)
            .unwrap();
        let state_before = fs::read(config.join(STATE)).unwrap();
        fs::remove_dir_all(repo.join("assets/skills/research")).unwrap();
        let installed = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        let error = installed.apply(&selected).unwrap_err().to_string();
        assert!(error.contains("north-explore requires missing bundled skill research"));
        assert_eq!(fs::read(config.join(STATE)).unwrap(), state_before);
        assert!(config.join("skills/north-plan").is_symlink());
        assert!(!config.join(".north-install.lock").exists());
        // Missing assets cannot prevent removal of an otherwise valid installation.
        installed.uninstall().unwrap();
        assert!(!exists(&config.join("skills")).unwrap());
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
        let initial = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
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
                .join("assets/opencode/instructions/core.md")])
        );
        let installed = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        assert!(installed.merging());
        installed.apply(&BTreeSet::new()).unwrap();
        assert_eq!(fs::read(config.join("opencode.json")).unwrap(), before);
        Installation::load(&repo, Tool::OpenCode, &config)
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
        let initial = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        initial.apply(&BTreeSet::new()).unwrap();
        for _ in 0..2 {
            Installation::load(&repo, Tool::OpenCode, &config)
                .unwrap()
                .apply_with_merge(&BTreeSet::new(), true)
                .unwrap();
            assert_eq!(
                fs::read_to_string(config.join("AGENTS.md")).unwrap(),
                "original"
            );
            assert!(!config.join(BACKUP).exists());
            assert!(config.join("opencode.json").exists());
            Installation::load(&repo, Tool::OpenCode, &config)
                .unwrap()
                .apply_with_merge(&BTreeSet::new(), false)
                .unwrap();
            assert!(config.join("AGENTS.md").is_symlink());
            assert_eq!(fs::read_to_string(config.join(BACKUP)).unwrap(), "original");
            assert!(!config.join("opencode.json").exists());
        }
        Installation::load(&repo, Tool::OpenCode, &config)
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
        Installation::load(&repo, Tool::OpenCode, &config)
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
            moved.join("assets/opencode/opencode.json"),
            r#"{"plugin":["north-v2"]}"#,
        )
        .unwrap();
        Installation::load(&moved, Tool::OpenCode, &config)
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
                .join("assets/opencode/instructions/core.md")])
        );
        Installation::load(&moved, Tool::OpenCode, &config)
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
                "symlink" => symlink(repo.join("assets/opencode/opencode.json"), &path).unwrap(),
                _ => {
                    fs::write(&path, "{}").unwrap();
                    fs::write(config.join("agents/north-worker.md"), "custom").unwrap();
                }
            }
            let before = fs::read(&path).unwrap();
            assert!(
                Installation::load(&repo, Tool::OpenCode, &config)
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
        let initial = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
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

        let installed = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        assert!(installed.selected_skills().contains(AUTO_COMMIT));
        installed.apply(&BTreeSet::new()).unwrap();
        let manual = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
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
        let manual = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        manual.apply(&BTreeSet::from([AUTO_COMMIT.into()])).unwrap();
        assert!(!exists(&config.join("skills/commit")).unwrap());
        let automatic = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
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
        Installation::load(&repo, Tool::OpenCode, &config)
            .unwrap()
            .apply(&BTreeSet::from(["two".into()]))
            .unwrap();
        add_commit_modes(&repo);
        let upgraded = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
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
            Installation::load(&repo, Tool::OpenCode, &config)
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
                Installation::load(&repo, Tool::OpenCode, &config)
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
        let command = repo.join("assets/opencode/commands/north.md");
        fs::remove_file(&command).unwrap();
        Installation::load(&repo, Tool::OpenCode, &config)
            .unwrap()
            .apply(&BTreeSet::new())
            .unwrap();
        fs::write(&command, "command").unwrap();
        Installation::load(&repo, Tool::OpenCode, &config)
            .unwrap()
            .apply(&BTreeSet::new())
            .unwrap();
        assert!(matches_link(
            &config.join("commands/north.md"),
            &command.canonicalize().unwrap()
        ));
        // Tracked commands can still be removed after their source is deleted.
        fs::remove_file(command).unwrap();
        Installation::load(&repo, Tool::OpenCode, &config)
            .unwrap()
            .uninstall()
            .unwrap();
        assert!(!exists(&config.join("commands")).unwrap());
    }

    #[test]
    fn selections_follow_links_and_new_skills_start_disabled_on_rerun() {
        let (_temp, repo, config) = fixture();
        let initial = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        assert_eq!(
            initial.selected_skills(),
            BTreeSet::from(["one".into(), "two".into()])
        );
        initial.apply(&BTreeSet::from(["two".into()])).unwrap();
        fs::create_dir_all(repo.join("assets/skills/three")).unwrap();
        fs::write(repo.join("assets/skills/three/SKILL.md"), "new").unwrap();
        let updated = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        assert_eq!(updated.selected_skills(), BTreeSet::from(["two".into()]));
        fs::remove_file(config.join("skills/two")).unwrap();
        assert!(
            Installation::load(&repo, Tool::OpenCode, &config)
                .unwrap()
                .selected_skills()
                .is_empty()
        );
    }

    #[test]
    fn uninstall_removes_tracked_assets_deleted_from_checkout() {
        let (_temp, repo, config) = fixture();
        let initial = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        initial.apply(&initial.skill_names()).unwrap();
        fs::remove_dir_all(repo.join("assets/skills/two")).unwrap();
        Installation::load(&repo, Tool::OpenCode, &config)
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
        let stale = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        let active = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        active.apply(&BTreeSet::new()).unwrap();
        assert!(stale.apply(&stale.skill_names()).is_err());
        let current = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        fs::create_dir(config.join(".north-install.lock")).unwrap();
        assert!(current.uninstall().is_err());
        assert!(exists(&config.join("AGENTS.md")).unwrap());
    }

    #[test]
    fn invalid_state_cannot_remove_paths_outside_config() {
        let (_temp, repo, config) = fixture();
        let initial = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        initial.apply(&initial.skill_names()).unwrap();
        let mut state = read_state(&config, Tool::OpenCode).unwrap().unwrap();
        state.links.insert(
            PathBuf::from("skills/../../outside"),
            repo.join("assets/skills/two"),
        );
        fs::write(config.join(STATE), serde_json::to_vec(&state).unwrap()).unwrap();
        assert!(Installation::load(&repo, Tool::OpenCode, &config).is_err());
    }

    #[test]
    fn claude_installation_links_claude_assets_and_shared_skills() {
        let (_temp, repo, config) = fixture();
        add_claude(&repo);
        add_pipeline(&repo);
        fs::create_dir_all(&config).unwrap();
        fs::write(config.join("CLAUDE.md"), "my rules").unwrap();
        let canonical = repo.canonicalize().unwrap();
        let initial = Installation::load(&repo, Tool::Claude, &config).unwrap();
        assert_eq!(initial.tool, Tool::Claude);
        assert_eq!(
            initial.skill_names(),
            Installation::load(&repo, Tool::OpenCode, &config)
                .unwrap()
                .skill_names()
        );
        initial
            .apply(&BTreeSet::from([NORTH_PIPELINE.into(), "one".into()]))
            .unwrap();
        assert!(matches_link(
            &config.join("CLAUDE.md"),
            &canonical.join("assets/claude/instructions/core.md")
        ));
        assert_eq!(
            fs::read_to_string(config.join("CLAUDE-backup.md")).unwrap(),
            "my rules"
        );
        assert!(!exists(&config.join("AGENTS.md")).unwrap());
        assert!(matches_link(
            &config.join("agents/north-worker.md"),
            &canonical.join("assets/claude/agents/north-worker.md")
        ));
        assert!(matches_link(
            &config.join("commands/north.md"),
            &canonical.join("assets/claude/commands/north.md")
        ));
        assert!(matches_link(
            &config.join("skills/north-plan"),
            &canonical.join("assets/skills/north-plan")
        ));
        assert!(matches_link(
            &config.join("skills/one"),
            &canonical.join("assets/skills/one")
        ));
        // Claude Code runs the pipeline skills as slash commands; no wrappers are linked.
        for name in PIPELINE_COMMANDS {
            assert!(!exists(&config.join("commands").join(name)).unwrap());
        }
        assert!(
            fs::read_to_string(config.join(STATE))
                .unwrap()
                .contains("\"tool\": \"claude\"")
        );
        assert!(Installation::load(&repo, Tool::OpenCode, &config).is_err());
        let installed = Installation::load(&repo, Tool::Claude, &config).unwrap();
        assert!(installed.installed());
        assert_eq!(
            installed.selected_skills(),
            BTreeSet::from([
                NORTH_PIPELINE.into(),
                "one".into(),
                "north-sources".into(),
                "clarify-requirements".into(),
                "research".into(),
                "subagent-usage".into(),
            ])
        );
        installed.uninstall().unwrap();
        assert_eq!(
            fs::read_to_string(config.join("CLAUDE.md")).unwrap(),
            "my rules"
        );
        assert!(!exists(&config.join("CLAUDE-backup.md")).unwrap());
        assert!(!exists(&config.join("skills")).unwrap());
    }

    #[test]
    fn claude_merge_imports_instructions_and_switches_modes_both_ways() {
        let (_temp, repo, config) = fixture();
        add_claude(&repo);
        fs::create_dir_all(&config).unwrap();
        fs::write(config.join("CLAUDE.md"), "# Mine\n").unwrap();
        let import = format!(
            "@{}",
            repo.canonicalize()
                .unwrap()
                .join("assets/claude/instructions/core.md")
                .display()
        );
        let merged = format!("# Mine\n{import}\n");
        Installation::load(&repo, Tool::Claude, &config)
            .unwrap()
            .apply_with_merge(&BTreeSet::new(), true)
            .unwrap();
        assert_eq!(
            fs::read_to_string(config.join("CLAUDE.md")).unwrap(),
            merged
        );
        assert!(!config.join("CLAUDE.md").is_symlink());
        assert!(!exists(&config.join("CLAUDE-backup.md")).unwrap());
        let state_before = fs::read(config.join(STATE)).unwrap();
        let installed = Installation::load(&repo, Tool::Claude, &config).unwrap();
        assert!(installed.merging());
        installed.apply(&BTreeSet::new()).unwrap();
        assert_eq!(fs::read(config.join(STATE)).unwrap(), state_before);
        assert_eq!(
            fs::read_to_string(config.join("CLAUDE.md")).unwrap(),
            merged
        );

        // Merge -> link keeps the import out of the backup.
        Installation::load(&repo, Tool::Claude, &config)
            .unwrap()
            .apply_with_merge(&BTreeSet::new(), false)
            .unwrap();
        assert!(config.join("CLAUDE.md").is_symlink());
        assert_eq!(
            fs::read_to_string(config.join("CLAUDE-backup.md")).unwrap(),
            "# Mine\n"
        );
        // Link -> merge restores the original and appends the import.
        Installation::load(&repo, Tool::Claude, &config)
            .unwrap()
            .apply_with_merge(&BTreeSet::new(), true)
            .unwrap();
        assert_eq!(
            fs::read_to_string(config.join("CLAUDE.md")).unwrap(),
            merged
        );
        assert!(!exists(&config.join("CLAUDE-backup.md")).unwrap());
        // Later user edits around the import survive uninstall.
        fs::write(
            config.join("CLAUDE.md"),
            format!("# Mine\n{import}\n\nLater edit\n"),
        )
        .unwrap();
        Installation::load(&repo, Tool::Claude, &config)
            .unwrap()
            .uninstall()
            .unwrap();
        assert_eq!(
            fs::read_to_string(config.join("CLAUDE.md")).unwrap(),
            "# Mine\n\nLater edit\n"
        );
        assert!(!exists(&config.join(STATE)).unwrap());
    }

    #[test]
    fn claude_merge_without_instructions_creates_and_removes_the_file() {
        let (_temp, repo, config) = fixture();
        add_claude(&repo);
        Installation::load(&repo, Tool::Claude, &config)
            .unwrap()
            .apply_with_merge(&BTreeSet::new(), true)
            .unwrap();
        let content = fs::read_to_string(config.join("CLAUDE.md")).unwrap();
        assert!(
            content.starts_with('@') && content.ends_with("assets/claude/instructions/core.md\n")
        );
        // Switching to link mode removes the file North created instead of backing it up.
        Installation::load(&repo, Tool::Claude, &config)
            .unwrap()
            .apply_with_merge(&BTreeSet::new(), false)
            .unwrap();
        assert!(config.join("CLAUDE.md").is_symlink());
        assert!(!exists(&config.join("CLAUDE-backup.md")).unwrap());
        Installation::load(&repo, Tool::Claude, &config)
            .unwrap()
            .apply_with_merge(&BTreeSet::new(), true)
            .unwrap();
        assert_eq!(
            fs::read_to_string(config.join("CLAUDE.md")).unwrap(),
            content
        );
        Installation::load(&repo, Tool::Claude, &config)
            .unwrap()
            .uninstall()
            .unwrap();
        assert!(!exists(&config.join("CLAUDE.md")).unwrap());
    }

    #[test]
    fn claude_merge_refuses_symlinked_instructions_without_changes() {
        let (_temp, repo, config) = fixture();
        add_claude(&repo);
        fs::create_dir_all(&config).unwrap();
        symlink(
            repo.join("assets/claude/instructions/core.md"),
            config.join("CLAUDE.md"),
        )
        .unwrap();
        assert!(
            Installation::load(&repo, Tool::Claude, &config)
                .unwrap()
                .apply_with_merge(&BTreeSet::new(), true)
                .is_err()
        );
        assert!(!exists(&config.join(STATE)).unwrap());
        assert!(!exists(&config.join("agents")).unwrap());
    }

    #[test]
    fn legacy_opencode_layout_links_and_state_migrate_to_tool_assets() {
        let (_temp, repo, config) = fixture();
        let canonical = repo.canonicalize().unwrap();
        // Links from the shell installer into the flat layout are adopted without state.
        fs::create_dir_all(config.join("skills")).unwrap();
        fs::create_dir_all(config.join("agents")).unwrap();
        symlink(
            canonical.join("assets/instructions/core.md"),
            config.join("AGENTS.md"),
        )
        .unwrap();
        symlink(
            canonical.join("assets/skills/one"),
            config.join("skills/one"),
        )
        .unwrap();
        symlink(
            canonical.join("assets/agents/north-worker.md"),
            config.join("agents/north-worker.md"),
        )
        .unwrap();
        let adopted = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        assert!(adopted.installed());
        assert_eq!(adopted.selected_skills(), BTreeSet::from(["one".into()]));
        adopted.apply(&BTreeSet::from(["two".into()])).unwrap();
        assert!(matches_link(
            &config.join("AGENTS.md"),
            &canonical.join("assets/opencode/instructions/core.md")
        ));
        assert!(matches_link(
            &config.join("agents/north-worker.md"),
            &canonical.join("assets/opencode/agents/north-worker.md")
        ));
        assert!(!exists(&config.join("skills/one")).unwrap());
        assert!(config.join("skills/two").is_symlink());

        // A state file written before the layout change validates and relinks.
        let mut state = read_state(&config, Tool::OpenCode).unwrap().unwrap();
        for (relative, source) in [
            ("AGENTS.md", "assets/instructions/core.md"),
            ("agents/north-worker.md", "assets/agents/north-worker.md"),
            ("commands/north.md", "assets/commands/north.md"),
        ] {
            let target = config.join(relative);
            fs::remove_file(&target).unwrap();
            symlink(canonical.join(source), &target).unwrap();
            state.links.insert(relative.into(), canonical.join(source));
        }
        let mut json = serde_json::to_value(&state).unwrap();
        json.as_object_mut().unwrap().remove("tool");
        fs::write(
            config.join(STATE),
            serde_json::to_vec_pretty(&json).unwrap(),
        )
        .unwrap();
        let upgraded = Installation::load(&repo, Tool::OpenCode, &config).unwrap();
        assert!(Installation::load(&repo, Tool::Claude, &config).is_err());
        upgraded.apply(&upgraded.selected_skills()).unwrap();
        for (relative, source) in [
            ("AGENTS.md", "assets/opencode/instructions/core.md"),
            (
                "agents/north-worker.md",
                "assets/opencode/agents/north-worker.md",
            ),
            ("commands/north.md", "assets/opencode/commands/north.md"),
        ] {
            assert!(matches_link(
                &config.join(relative),
                &canonical.join(source)
            ));
        }
        let current = read_state(&config, Tool::OpenCode).unwrap().unwrap();
        assert_eq!(current.tool, Tool::OpenCode);
        // Claude Code never linked the flat layout, so it does not adopt those sources.
        let mut bad = current.clone();
        bad.tool = Tool::Claude;
        bad.links.insert(
            "CLAUDE.md".into(),
            canonical.join("assets/instructions/core.md"),
        );
        fs::write(config.join(STATE), serde_json::to_vec(&bad).unwrap()).unwrap();
        assert!(Installation::load(&repo, Tool::Claude, &config).is_err());
        fs::write(config.join(STATE), serde_json::to_vec(&current).unwrap()).unwrap();
        Installation::load(&repo, Tool::OpenCode, &config)
            .unwrap()
            .uninstall()
            .unwrap();
        assert!(!exists(&config.join("AGENTS.md")).unwrap());
    }
}
