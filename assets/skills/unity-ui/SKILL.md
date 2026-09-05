---
name: unity-ui
description: Create or modify Unity user interfaces as persistent, editable scene, prefab, or UI Toolkit assets through the Unity Editor, rather than constructing the interface in runtime code. Use for requests to build, lay out, style, or change Unity menus, HUDs, panels, dialogs, and UI controls. Does not apply to unrelated Unity gameplay code or explanations of existing UI code without a request to change it.
---

# Unity UI

Author the requested interface in the Unity Editor so its hierarchy, layout,
styling, and references are saved in project assets and remain editable before
entering Play Mode. Inspect the existing scene, prefabs, and UI system first;
extend the project's established approach instead of switching UI frameworks.

- For uGUI, create and configure the Canvas, UI objects, RectTransforms,
  components, and prefab instances in the Editor. Save scene or prefab changes.
- For UI Toolkit, use UI Builder to author visual structure and styles in UXML
  and USS assets, and configure the associated UIDocument in the Editor.

Use available Unity Editor integration tools or direct Editor interaction.
Editor-only automation is acceptable when it authors and saves ordinary scene,
prefab, or UI assets; do not leave UI creation dependent on a script running
when the game starts. Do not hand-edit serialized scene or prefab YAML as a
substitute for Editor authoring.

Keep runtime scripts focused on behavior: event handling, data binding,
navigation, and updating existing controls. Do not build the static hierarchy,
layout, or styling through runtime GameObject creation, AddComponent calls,
programmatic VisualElement trees, or OnGUI. For dynamic lists or repeated
elements, instantiate Editor-authored prefabs or UXML templates and bind data
to them instead of recreating their structure in code.

If Editor access is unavailable, state that limitation and provide concrete
Editor steps or request the needed access. Do not silently substitute a runtime
UI generator. Follow an explicit user request for a different approach, making
the departure from Editor authoring clear.

Verify the saved interface in Edit Mode and test its requested interactions in
Play Mode when available. Check the relevant target resolutions, layout behavior,
references, and Console errors. Report the assets changed and what was actually
verified; distinguish untested behavior from confirmed results.
