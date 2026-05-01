# Franz UI Navigation State Machine

Dieses Dokument beschreibt die komplette UI-Navigation als Mermaid-Diagramme:
- alle Haupt-Screens,
- Fokuswechsel,
- Dialog-Öffnungen,
- Dialog-interne Key-Transitions,
- Skills-Manager-Fokusmodell (Input/Actions/List) zur Vermeidung von Focus-Traps.

Stand: Codebasis in `internal/ui/model/ui.go`, `internal/ui/model/keys.go`, `internal/ui/dialog/*`.

Normativer Fokus-Standard fuer alle aktuellen und zukuenftigen Screens:
`docs/focus-standard.md`.

## 0) Vollständige Screen-/Overlay-Inventarliste

```mermaid
flowchart TD
    App[Franz TUI] --> CoreScreens
    App --> GlobalLayers
    App --> Dialogs

    CoreScreens --> Onboarding[Onboarding Screen]
    CoreScreens --> Initialize[Initialize Screen]
    CoreScreens --> Landing[Landing Screen]
    CoreScreens --> ChatWide[Chat Screen Wide]
    CoreScreens --> ChatCompact[Chat Screen Compact]
    ChatCompact --> SessionDetailsOverlay[Compact Session Details Overlay]

    GlobalLayers --> Header[Header]
    GlobalLayers --> StatusBar[Status/Help Bar]
    GlobalLayers --> CompletionsPopup[@-Completions Popup]
    GlobalLayers --> EditorAttachments[Editor Attachments Row]

    Dialogs --> CommandsDlg[Commands Dialog]
    Dialogs --> ModelsDlg[Models Dialog]
    Dialogs --> SessionsDlg[Sessions Dialog]
    Dialogs --> ReasoningDlg[Reasoning Dialog]
    Dialogs --> SkillsDlg[Skills Manager Dialog]
    Dialogs --> FilePickerDlg[File Picker Dialog]
    Dialogs --> PermissionsDlg[Permissions Dialog]
    Dialogs --> OAuthDlg[OAuth Dialogs]
    Dialogs --> ApiKeyDlg[API Key Input Dialog]
    Dialogs --> QuitDlg[Quit Confirm Dialog]
    Dialogs --> ArgumentsDlg[Arguments Dialog]
```

## 0.1) Render-Komposition pro Hauptscreen

```mermaid
flowchart LR
    Onboarding --> HeaderA[Header]
    Onboarding --> StatusA[Status/Help]
    Onboarding --> DialogOverlayA[Dialog Overlay]

    Initialize --> HeaderB[Header]
    Initialize --> MainInit[Initialize View]
    Initialize --> StatusB[Status/Help]
    Initialize --> DialogOverlayB[Dialog Overlay]

    Landing --> HeaderC[Header]
    Landing --> MainLanding[Landing View]
    Landing --> EditorC[Editor + Attachments]
    Landing --> StatusC[Status/Help]
    Landing --> DialogOverlayC[Dialog Overlay]

    ChatWide --> Sidebar[Right Sidebar]
    ChatWide --> ChatMain[Chat Main View]
    ChatWide --> Pills[Todo Pills Optional]
    ChatWide --> EditorWide[Editor + Attachments]
    ChatWide --> StatusD[Status/Help]
    ChatWide --> CompletionsD[Completions Popup Optional]
    ChatWide --> DialogOverlayD[Dialog Overlay]

    ChatCompact --> HeaderE[Header Compact]
    ChatCompact --> ChatMainE[Chat Main View]
    ChatCompact --> PillsE[Todo Pills Optional]
    ChatCompact --> EditorE[Editor + Attachments]
    ChatCompact --> DetailsE[Details Overlay Optional]
    ChatCompact --> StatusE[Status/Help]
    ChatCompact --> CompletionsE[Completions Popup Optional]
    ChatCompact --> DialogOverlayE[Dialog Overlay]
```

## 1) App-Level State Machine (Screens + Globale Shortcuts)

```mermaid
stateDiagram-v2
    [*] --> Onboarding: app start + not configured
    [*] --> Initialize: app start + init required
    [*] --> Landing: app start + configured

    Onboarding --> Landing: model selected/auth completed
    Initialize --> Landing: init flow finished
    Landing --> Chat: load/new session
    Chat --> Landing: session closed/unloaded

    state Landing {
        [*] --> EditorFocus
        EditorFocus --> MainFocus: Tab
        MainFocus --> EditorFocus: Tab
    }

    state Chat {
        [*] --> EditorFocus
        EditorFocus --> MainFocus: Tab
        MainFocus --> EditorFocus: Tab
    }

    state GlobalOverlays {
        [*] --> NoDialog
        NoDialog --> QuitDialog: Ctrl+C
        NoDialog --> CommandsDialog: Ctrl+P (or "/" on empty editor)
        NoDialog --> ModelsDialog: Ctrl+M / Ctrl+L
        NoDialog --> SessionsDialog: Ctrl+S
        NoDialog --> SidebarToggle: Ctrl+B (chat mode)
        NoDialog --> HelpToggle: Ctrl+G
        NoDialog --> ModeCycle: Shift+Tab
        NoDialog --> YoloToggle: Ctrl+Y
        NoDialog --> PlanToggle: Ctrl+Shift+P
        NoDialog --> Suspend: Ctrl+Z
    }

    state InlineOverlays {
        [*] --> None
        None --> CompletionsPopup: "@" completion trigger
        CompletionsPopup --> None: space / cursor move / selection complete
        None --> AttachmentsRow: attachment exists
        AttachmentsRow --> AttachmentDeleteMode: Ctrl+R
        AttachmentDeleteMode --> AttachmentsRow: Esc / delete op done
    }

    state EditorInputKeys {
        [*] --> Idle
        Idle --> SendMessage: Enter
        Idle --> Newline: Shift+Enter / Ctrl+J
        Idle --> OpenExternalEditor: Ctrl+O
        Idle --> AddImage: Ctrl+F
        Idle --> PasteImage: Ctrl+V
        Idle --> PrevWord: Ctrl+Left / Alt+B
        Idle --> NextWord: Ctrl+Right / Alt+F
        Idle --> ClearInput: Ctrl+Backspace
        Idle --> HistoryPrev: Up
        Idle --> HistoryNext: Down
        Idle --> AttachmentDeleteMode: Ctrl+R
    }
```

## 1.1) Vollständiger Dialog-Öffnungsgraph

```mermaid
flowchart TD
    NoDialog[No Dialog Open] -->|Ctrl+P| CommandsDialog
    NoDialog -->|Ctrl+L / Ctrl+M| ModelsDialog
    NoDialog -->|Ctrl+S| SessionsDialog
    NoDialog -->|Ctrl+C| QuitDialog
    NoDialog -->|Tool permission request| PermissionsDialog
    NoDialog -->|Auth required| OAuthOrApiKeyDialog
    NoDialog -->|Ctrl+F in editor| FilePickerDialog

    CommandsDialog -->|Switch Model| ModelsDialog
    CommandsDialog -->|Sessions| SessionsDialog
    CommandsDialog -->|Select Reasoning| ReasoningDialog
    CommandsDialog -->|Skills Manager| SkillsDialog
    CommandsDialog -->|Open File Picker| FilePickerDialog
    CommandsDialog -->|Quit| QuitDialog

    ModelsDialog -->|Re-auth needed| OAuthOrApiKeyDialog
    SkillsDialog -->|Open Link| ExternalBrowser
```

## 2) Dialog Routing (welcher Screen öffnet welchen Dialog)

```mermaid
flowchart TD
    A[Main UI Loop] --> B{Dialog offen?}
    B -- Nein --> C[Normale Screen-Keymaps]
    B -- Ja --> D[handleDialogMsg]

    C --> E[Commands Dialog öffnen]
    C --> F[Models Dialog öffnen]
    C --> G[Sessions Dialog öffnen]
    C --> H[Quit Dialog öffnen]
    C --> I[File Picker öffnen]
    C --> J[Reasoning Dialog öffnen]
    C --> K[Skills Manager öffnen]
    C --> L[OAuth/API-Key Dialog öffnen]
    C --> M[Permissions Dialog öffnen]

    E --> N[Action aus Commands]
    N --> F
    N --> G
    N --> J
    N --> I
    N --> K
    N --> H
    N --> O[Toggle/Run Action]

    L --> P[Auth Complete]
    P --> A
    M --> Q[Allow / Allow Session / Discuss / Deny]
    Q --> A
```

## 3) Commands Dialog (System/User/MCP)

```mermaid
stateDiagram-v2
    [*] --> SystemCommands
    SystemCommands --> UserCommands: Tab
    UserCommands --> MCPPrompts: Tab
    MCPPrompts --> SystemCommands: Tab

    SystemCommands --> MCPPrompts: Shift+Tab
    MCPPrompts --> UserCommands: Shift+Tab
    UserCommands --> SystemCommands: Shift+Tab

    state ListNav {
        [*] --> Idle
        Idle --> MoveDown: Down
        Idle --> MoveUp: Up / Ctrl+P
        Idle --> Execute: Enter / Ctrl+Y
        Idle --> Close: Esc
    }
```

## 4) Models Dialog

```mermaid
stateDiagram-v2
    [*] --> ModelList
    ModelList --> MoveDown: Down / Ctrl+N
    ModelList --> MoveUp: Up / Ctrl+P
    ModelList --> ToggleModelType: Tab / Shift+Tab
    ModelList --> SelectModel: Enter / Ctrl+Y
    ModelList --> EditModel: Ctrl+E
    ModelList --> Close: Esc
```

## 5) Sessions Dialog

```mermaid
stateDiagram-v2
    [*] --> Normal
    Normal --> SelectSession: Enter / Tab / Ctrl+Y
    Normal --> MoveDown: Down / Ctrl+N
    Normal --> MoveUp: Up / Ctrl+P
    Normal --> RenameMode: Ctrl+R
    Normal --> DeleteConfirm: Ctrl+X
    Normal --> Close: Esc

    RenameMode --> Normal: Enter (confirm)
    RenameMode --> Normal: Esc (cancel)

    DeleteConfirm --> Normal: Y (confirm)
    DeleteConfirm --> Normal: N / Esc (cancel)
```

## 6) Skills Manager (Wichtig gegen Focus-Traps)

### 6.1 Top-Level Layout/Fokus

```mermaid
stateDiagram-v2
    [*] --> InstalledTab
    InstalledTab --> SearchTab: Right
    SearchTab --> InstalledTab: Left
    InstalledTab --> SearchTab: Tab (tab switch view)
    SearchTab --> InstalledTab: Tab (tab switch view)

    state SearchTab {
        [*] --> InputFocus
        InputFocus --> ActionsFocus: Tab
        ActionsFocus --> ListFocus: Tab
        ListFocus --> InputFocus: Tab

        ListFocus --> ActionsFocus: Esc
        ActionsFocus --> InputFocus: Esc
        InputFocus --> CloseDialog: Esc
    }

    state InstalledTab {
        [*] --> ListFocusInstalled
        ListFocusInstalled --> ActionsFocusInstalled: Tab
        ActionsFocusInstalled --> ListFocusInstalled: Tab
    }
```

### 6.2 Search Tab Key-Transitions

```mermaid
stateDiagram-v2
    [*] --> InputFocus

    state InputFocus {
        [*] --> Typing
        Typing --> DebouncedSearch: any char incl. space
        Typing --> ClearInput: Ctrl+Backspace
        Typing --> ActionsFocus: Tab / Down
        Typing --> InstalledTab: Left
        Typing --> SearchTab: Right
    }

    state ActionsFocus {
        [*] --> ActionSelected
        ActionSelected --> PrevAction: Ctrl+Left
        ActionSelected --> NextAction: Ctrl+Right
        ActionSelected --> RunAction: Enter
        ActionSelected --> ListFocus: Down
        ActionSelected --> InputFocus: Up / Esc
    }

    state ListFocus {
        [*] --> ItemSelected
        ItemSelected --> MoveUp: Up
        ItemSelected --> MoveDown: Down
        ItemSelected --> ToggleMultiSelect: Space
        ItemSelected --> InputFocus: any typing key
        ItemSelected --> ActionsFocus: Esc
    }
```

### 6.3 Installed Tab Key-Transitions

```mermaid
stateDiagram-v2
    [*] --> ListFocus
    ListFocus --> ActionsFocus: Tab
    ActionsFocus --> ListFocus: Tab

    state ListFocus {
        [*] --> SelectedItem
        SelectedItem --> MoveUp: Up
        SelectedItem --> MoveDown: Down
        SelectedItem --> ToggleMultiSelect: Space
        SelectedItem --> ToggleDetails: Enter
    }

    state ActionsFocus {
        [*] --> ActionSelected
        ActionSelected --> PrevAction: Ctrl+Left
        ActionSelected --> NextAction: Ctrl+Right
        ActionSelected --> RunAction: Enter
    }
```

### 6.4 Skills Actions (auf selektierte Items)

```mermaid
flowchart TD
    A[Installed List selected items] --> B{Action}
    B -->|Enable| C[ActionSkillsSetDisabledBatch Disabled=false]
    B -->|Disable| D[ActionSkillsSetDisabledBatch Disabled=true]
    B -->|Fix Perms| E[ActionSkillsFixPerms Names=[]]
    B -->|Delete| F[ActionSkillsDeleteBatch Names=[]]
    B -->|Refresh| G[Reload installed list]
    B -->|Sources| H[Load tracked sources]

    I[Search List selected sources] --> J{Action}
    J -->|Details| K[Toggle details panel]
    J -->|Install| L[Queue install step-by-step]
    J -->|Open Link| M[Open details URL]
    J -->|Refresh| N[Search again]
    J -->|Sources| O[Load tracked sources]
```

## 7) Permissions Dialog (Tool Calls)

```mermaid
stateDiagram-v2
    [*] --> PermissionPrompt
    PermissionPrompt --> SelectAllow: A / Ctrl+A
    PermissionPrompt --> SelectAllowSession: S / Ctrl+S
    PermissionPrompt --> SelectDiscuss: C / Ctrl+C
    PermissionPrompt --> SelectDeny: D
    PermissionPrompt --> MoveOption: Left / Right / Tab
    PermissionPrompt --> ConfirmCurrent: Enter / Ctrl+Y
    PermissionPrompt --> ToggleDiffMode: T
    PermissionPrompt --> ToggleFullscreen: F
    PermissionPrompt --> ScrollDiff: Shift+Arrow / HJKL
    PermissionPrompt --> Close: Esc
```

## 8) Quit Dialog

```mermaid
stateDiagram-v2
    [*] --> NoSelected
    NoSelected --> YesSelected: Left / Right / Tab
    YesSelected --> NoSelected: Left / Right / Tab

    NoSelected --> Close: Enter / Space / N
    YesSelected --> Quit: Enter / Space / Y / Ctrl+C
```

## 9) OAuth / API-Key Dialogs

```mermaid
stateDiagram-v2
    [*] --> OAuthOrApiKey
    OAuthOrApiKey --> Submit: Enter / Ctrl+Y
    OAuthOrApiKey --> CopyCode: C (OAuth)
    OAuthOrApiKey --> Close: Esc
    Submit --> AuthSuccess
    Submit --> AuthError
    AuthSuccess --> [*]
    AuthError --> OAuthOrApiKey
```

## 10) File Picker Dialog

```mermaid
stateDiagram-v2
    [*] --> Browse
    Browse --> Up: Up / K
    Browse --> Down: Down / J
    Browse --> Forward: Right / L
    Browse --> Backward: Left / H
    Browse --> Select: Enter
    Browse --> Close: Esc
```

## 10.1) Reasoning Dialog

```mermaid
stateDiagram-v2
    [*] --> ReasoningList
    ReasoningList --> MoveDown: Down / Ctrl+N
    ReasoningList --> MoveUp: Up / Ctrl+P
    ReasoningList --> SelectEffort: Enter / Ctrl+Y
    ReasoningList --> Close: Esc
```

## 10.2) Arguments Dialog

```mermaid
stateDiagram-v2
    [*] --> ArgumentFields
    ArgumentFields --> NextField: Down / Tab
    ArgumentFields --> PrevField: Up / Shift+Tab
    ArgumentFields --> Confirm: Enter
    ArgumentFields --> Close: Esc
```

## 11) Sequenzdiagramm: Key Event Routing (zentral)

```mermaid
sequenceDiagram
    participant User
    participant BubbleTea as BubbleTea Loop
    participant UI as UI Model
    participant Overlay as Dialog Overlay
    participant App as App Services

    User->>BubbleTea: KeyPress
    BubbleTea->>UI: Update(msg)
    UI->>UI: Global quit check (Ctrl+C)
    alt Any dialog open
        UI->>Overlay: dialog.Update(msg)
        Overlay-->>UI: Action
        UI->>App: Execute action (if needed)
    else No dialog open
        UI->>UI: Screen/focus key handling
        alt command opens dialog
            UI->>Overlay: Open dialog
        else executes command/tool
            UI->>App: Run command/tool action
        end
    end
    UI-->>BubbleTea: Cmd / state
```

## 12) Regelwerk zur Vermeidung von Focus-Traps (Normativ)

```mermaid
flowchart TD
    A[Für jeden Screen/Dialog] --> B[Definiere eindeutige Fokus-Hierarchie]
    B --> C[Tab = nächster Fokus innerhalb View]
    C --> D[Shift+Tab = vorheriger Fokus innerhalb View]
    D --> E[Esc = eine Ebene zurück]
    E --> F[Nur Root-Ebene darf Dialog schließen]
    F --> G[Arrow-Keys nur 1 Verantwortung je Fokus]
    G --> H[Space nur Toggle in List-Fokus]
    H --> I[Typing-Keys nur im Input-Fokus]
```

---

Wenn du willst, kann ich als nächsten Schritt aus diesem Diagramm eine
`UI_NAVIGATION_CONTRACT.md` mit testbaren Invarianten machen (z. B. "Esc from
list must never close dialog directly"), damit Focus-Traps per Tests geblockt
werden.
