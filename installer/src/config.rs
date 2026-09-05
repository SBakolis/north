use anyhow::{Context, Result, ensure};
use jsonc_parser::{
    ParseOptions,
    cst::{CstInputValue, CstNode, CstObject, CstRootNode},
};
use serde::{Deserialize, Serialize};
use serde_json::{Value, json};
use std::collections::BTreeSet;

// Save both versions so uninstall can undo our additions without losing later edits.
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct ConfigMerge {
    pub file: String,
    pub original: Option<String>,
    pub applied: String,
}

fn parse(text: &str) -> Result<(CstRootNode, Value)> {
    let options = ParseOptions {
        allow_loose_object_property_names: false,
        ..Default::default()
    };
    let root = CstRootNode::parse(text, &options).context("Invalid OpenCode JSON/JSONC")?;
    ensure!(
        root.object_value().is_some(),
        "OpenCode configuration must be an object"
    );
    validate_keys(root.value().unwrap())?;
    let value = jsonc_parser::parse_to_serde_value(text, &options)?.unwrap();
    Ok((root, value))
}

fn validate_keys(node: CstNode) -> Result<()> {
    if let Some(object) = node.as_object() {
        let mut names = BTreeSet::new();
        for prop in object.properties() {
            let name = prop
                .name()
                .unwrap()
                .decoded_value()
                .map_err(|_| anyhow::anyhow!("Invalid configuration key"))?;
            ensure!(
                names.insert(name.clone()),
                "Duplicate configuration key: {name}"
            );
            validate_keys(prop.value().unwrap())?;
        }
    } else if let Some(array) = node.as_array() {
        for child in array.elements() {
            validate_keys(child)?;
        }
    }
    Ok(())
}

// Existing scalar settings win. Objects combine recursively; arrays form a union.
fn merge(current: &mut Value, north: &Value) {
    match (current, north) {
        (Value::Object(current), Value::Object(north)) => {
            for (key, value) in north {
                match current.get_mut(key) {
                    Some(existing) => merge(existing, value),
                    None => {
                        current.insert(key.clone(), value.clone());
                    }
                }
            }
        }
        (Value::Array(current), Value::Array(north)) => {
            for value in north {
                if !current.contains(value) {
                    current.push(value.clone());
                }
            }
        }
        _ => {}
    }
}

fn input(value: &Value) -> CstInputValue {
    match value {
        Value::Null => CstInputValue::Null,
        Value::Bool(value) => CstInputValue::Bool(*value),
        Value::Number(value) => CstInputValue::Number(value.to_string()),
        Value::String(value) => CstInputValue::String(value.clone()),
        Value::Array(values) => CstInputValue::Array(values.iter().map(input).collect()),
        Value::Object(values) => CstInputValue::Object(
            values
                .iter()
                .map(|(key, value)| (key.clone(), input(value)))
                .collect(),
        ),
    }
}

// Only touch changed properties/elements, retaining comments and unrelated formatting.
fn patch(object: CstObject, before: &Value, after: &Value) {
    for key in before.as_object().unwrap().keys() {
        if after.get(key).is_none() {
            object.get(key).unwrap().remove();
        }
    }
    for (key, next) in after.as_object().unwrap() {
        let Some(previous) = before.get(key) else {
            object.append(key, input(next));
            continue;
        };
        if previous == next {
            continue;
        }
        let prop = object.get(key).unwrap();
        match (previous, next) {
            (Value::Object(_), Value::Object(_)) => {
                patch(prop.object_value().unwrap(), previous, next)
            }
            (Value::Array(previous), Value::Array(next)) => {
                let array = prop.array_value().unwrap();
                for (node, value) in array.elements().into_iter().zip(previous) {
                    if !next.contains(value) {
                        node.remove();
                    }
                }
                for value in next {
                    if !previous.contains(value) {
                        array.append(input(value));
                    }
                }
            }
            _ => prop.set_value(input(next)),
        }
    }
}

// A changed scalar belongs to the user. Only remove still-matching North additions.
fn subtract(current: &Value, original: Option<&Value>, applied: &Value) -> Option<Value> {
    if original == Some(applied) {
        return Some(current.clone());
    }
    if current == applied {
        return original.cloned();
    }
    match (current, applied) {
        (Value::Object(current), Value::Object(applied)) => {
            let mut result = current.clone();
            for (key, value) in applied {
                if let Some(current) = current.get(key) {
                    match subtract(current, original.and_then(|v| v.get(key)), value) {
                        Some(value) => {
                            result.insert(key.clone(), value);
                        }
                        None => {
                            result.remove(key);
                        }
                    }
                }
            }
            Some(Value::Object(result))
        }
        (Value::Array(current), Value::Array(applied)) => {
            let original = original.and_then(Value::as_array);
            Some(Value::Array(
                current
                    .iter()
                    .filter(|value| {
                        !applied.contains(value)
                            || original.is_some_and(|items| items.contains(value))
                    })
                    .cloned()
                    .collect(),
            ))
        }
        _ => Some(current.clone()),
    }
}

impl ConfigMerge {
    pub fn new(file: String, original: Option<String>, north: &Value) -> Result<Self> {
        ensure!(north.is_object(), "North configuration must be an object");
        let (root, before) = parse(original.as_deref().unwrap_or("{}\n"))?;
        // These OpenCode fields must be arrays for North's additions to take effect.
        for key in ["instructions", "plugin"] {
            if north.get(key).is_some() {
                ensure!(north[key].is_array(), "North {key} must be an array");
                ensure!(
                    before.get(key).is_none_or(Value::is_array),
                    "Existing {key} must be an array"
                );
            }
        }
        let mut after = before.clone();
        merge(&mut after, north);
        patch(root.object_value().unwrap(), &before, &after);
        Ok(Self {
            file,
            original,
            applied: root.to_string(),
        })
    }

    pub fn remove(&self, current: Option<&str>) -> Result<Option<String>> {
        let Some(current) = current else {
            return Ok(None);
        };
        if current == self.applied {
            return Ok(self.original.clone());
        }
        let (root, value) = parse(current)?;
        let original = parse(self.original.as_deref().unwrap_or("{}"))?.1;
        let applied = parse(&self.applied)?.1;
        let after = subtract(&value, Some(&original), &applied).unwrap_or(json!({}));
        patch(root.object_value().unwrap(), &value, &after);
        Ok(Some(root.to_string()))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn nested_settings_plugins_comments_and_original_bytes_survive() {
        let original = "{\n  // My setup\n  \"model\": \"custom\",\n  \"plugin\": [\"existing\", /* keep */ \"shared\",],\n  \"permission\": {\"edit\": \"ask\",},\n  \"instructions\": [\"my-rules.md\"],\n}\n";
        let north = json!({"model":"north", "plugin":["shared","north-plugin","north-plugin"], "permission":{"edit":"allow","bash":"ask"}, "instructions":["north.md"]});
        let merged =
            ConfigMerge::new("opencode.jsonc".into(), Some(original.into()), &north).unwrap();
        let value = parse(&merged.applied).unwrap().1;
        assert_eq!(value["model"], "custom");
        assert_eq!(value["permission"], json!({"edit":"ask","bash":"ask"}));
        assert_eq!(
            value["plugin"],
            json!(["existing", "shared", "north-plugin"])
        );
        assert_eq!(value["instructions"], json!(["my-rules.md", "north.md"]));
        assert!(merged.applied.contains("// My setup"));
        assert!(merged.applied.contains("/* keep */"));
        assert_eq!(
            merged.remove(Some(&merged.applied)).unwrap().as_deref(),
            Some(original)
        );
        let repeated =
            ConfigMerge::new(merged.file.clone(), Some(merged.applied.clone()), &north).unwrap();
        assert_eq!(repeated.applied, merged.applied);
    }

    #[test]
    fn uninstall_preserves_later_user_edits_and_preexisting_plugin() {
        let original = r#"{"plugin":["shared"],"permission":{"edit":"ask"}}"#;
        let north =
            json!({"plugin":["shared","north-plugin"],"permission":{"bash":"ask"},"model":"north"});
        let merged =
            ConfigMerge::new("opencode.json".into(), Some(original.into()), &north).unwrap();
        let edited = "{\n// later edit\n\"plugin\":[\"shared\",\"north-plugin\",\"later\"],\"permission\":{\"edit\":\"deny\",\"bash\":\"allow\"},\"model\":\"mine\",\"theme\":\"dark\"}";
        let removed = merged.remove(Some(edited)).unwrap().unwrap();
        assert!(removed.contains("// later edit"));
        assert_eq!(
            parse(&removed).unwrap().1,
            json!({"plugin":["shared","later"],"permission":{"edit":"deny","bash":"allow"},"model":"mine","theme":"dark"})
        );
    }

    #[test]
    fn new_configuration_and_new_nested_objects_can_be_removed() {
        let north = json!({"plugin":["north-plugin"],"mcp":{"north":{"enabled":true}},"instructions":["north.md"]});
        let merged = ConfigMerge::new("opencode.json".into(), None, &north).unwrap();
        assert!(serde_json::from_str::<Value>(&merged.applied).is_ok());
        assert_eq!(merged.remove(Some(&merged.applied)).unwrap(), None);
        let edited = r#"{"plugin":["north-plugin","user-plugin"],"mcp":{"north":{"enabled":true},"user":{"enabled":false}},"instructions":["north.md"]}"#;
        let removed = merged.remove(Some(edited)).unwrap().unwrap();
        assert_eq!(
            parse(&removed).unwrap().1,
            json!({"plugin":["user-plugin"],"mcp":{"user":{"enabled":false}}})
        );
    }

    #[test]
    fn malformed_ambiguous_and_incompatible_configurations_fail() {
        for invalid in [
            "{",
            "[]",
            "null",
            "",
            "// comment only",
            r#"{"instructions":false}"#,
            r#"{"plugin":{}}"#,
            r#"{"model":"a","model":"b"}"#,
            r#"{"nested":{"x":1,"x":2}}"#,
        ] {
            assert!(
                ConfigMerge::new(
                    "opencode.jsonc".into(),
                    Some(invalid.into()),
                    &json!({"instructions":["north.md"],"plugin":[]})
                )
                .is_err(),
                "accepted {invalid}"
            );
        }
    }
}
