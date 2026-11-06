# KNetTUI

The package provides a terminal user interface (TUI) for a KNet node: a console with the journal, panels for peers, metrics and node, and a command input line. The package encapsulates the application lifecycle, background processes for reading the journal and responses, execution of external programs, and screen rendering. Interaction with the KNet core goes through the `Commands.InterfaceKNet` and `Commands.InterfaceKNetCore` interfaces. Custom logic is plugged in via the `AppLogic` callback. Journal and API settings are passed as a single `KNTSettings` object.

## Settings

The module requires a set of data describing the journal and the core API to start.

| Name | Type | Default | Description |
|---|---|---|---|
| `Journal` | `KJournal.KJSettings` | — | Journal settings: writers, readers, buffers |
| `API` | `KNet.KNSettings` | — | KNet core settings: node, peers, commands, journal |

`KNTSettings` is validated by the `validate` method before startup; on error `Run` returns it and does not start.

## Lifecycle

### Start

Start is performed by the `Run` method. First it checks that `status` equals `OFF`, otherwise it returns an error. Then settings are validated, the journal is initialized, a writer and the `tuiJournal` reader are created, the design is set up and TUI processes are started. After one second, in a separate goroutine, the KNet core starts, along with its processes (`readerResponse`, `executerPrograms`, `readerPackages`, `updPeers`, `updMetrics`), and `appWorkstation` is called with the interfaces and the API context. The main thread blocks on `app.Run()`. The status goes through `RUNNING` → `ON`.

### Stop

Stop is performed by the `stop` method, which is triggered by `Esc`. The logic is guarded by two `once` values (`stopOnce`, `stopTwice`) and a `WaitGroup` (`awaitStopOnce`) to correctly handle a repeated call. The first call cancels `ctxAPI`, waits for `processesAPI` and calls `kn.Endwork()`. The second call waits for the first to finish, cancels `ctxTUI`, waits for `processesTUI`, closes the journal and after one second stops `app`. The status goes through `STOP` → `OFF`.

## Interaction

### Adding a page

The `AddPage` method adds a page to `tview.Pages` and binds a hotkey. The handler is stored in `inputHandlers` under the page key. The page is added via `QueueUpdateDraw`, while the handler is written outside of it. On a key press, `SetInputCapture` iterates over the handlers and switches the page. The method is safe when `app` or `pages` is `nil` — it simply returns. Removal is performed by the `DelPage` method, which deletes the handler and the page.

### Printing to console

The `PrintConsole` method writes a line to `console` and scrolls to the end. Writing is done via `QueueUpdateDraw`, so the method is thread-safe. Line formatting is handled by the `coloredRecord`, `coloredResponse`, `coloredPackage`, `ColoredText`, `LevelText`, `CropText`, `ChanText` helpers. Colors are defined as constants of type `Color`. Printing is used by the journal, response and package reader processes.

### Executing commands

Commands are entered in the `input` line and passed to `kn.Execute` on `Enter`. The `CommandsMap` is extended by the `setTUICommands` method, which registers `/clear`. External programs are executed by `executerPrograms`: on cursor change it takes `knt.kn.ExternalPrograms()`, finds the container in `CommandsMap` for each, calls `Check` and `Body`, and writes the response via `WriteResponse`. Hotkey handlers are stored in `inputHandlers` and invoked in `SetInputCapture`.

### Updating panels

Panels are updated by background processes. `updPeers` polls `GetPeerList` on a 200 ms ticker and updates `peers` via `QueueUpdateDraw`. `updMetrics` collects journal, KNet and KNetCore metrics on a 400 ms ticker and updates `metrics`. `updNode` is called once at API startup and fills `node` directly, without `QueueUpdateDraw`. `readerRecords` reads the journal and prints records, `readerResponse` — responses, `readerPackages` — packages.

## Example

```go
commands := make(map[string]Commands.CommandContainer)
knt := KNetTUI.NewKNetTUI(commands)

settings := KNTSettings{
    Journal: KJournal.KJSettings{ /* ... */ },
    API:     KNet.KNSettings{ /* ... */ },
}

workstation := func(i Interface, ctx context.Context, wg *sync.WaitGroup) {
    i.KNetTUI.PrintConsole("custom logic started")
}

if err := knt.Run(settings, workstation); err != nil {
    log.Fatal(err)
}
```

---

 - This documentation is also available in [Russian](./src/docs/README.ru.md).