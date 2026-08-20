<script lang="ts">
  import { onDestroy, onMount, tick } from "svelte";
  import { EditorState, StateEffect, StateField } from "@codemirror/state";
  import {
    defaultKeymap,
    history,
    historyKeymap,
    indentWithTab,
  } from "@codemirror/commands";
  import { markdown } from "@codemirror/lang-markdown";
  import {
    defaultHighlightStyle,
    syntaxHighlighting,
  } from "@codemirror/language";
  import {
    Decoration,
    EditorView,
    WidgetType,
    drawSelection,
    keymap,
  } from "@codemirror/view";
  import * as Y from "yjs";

  import { ensureDefaultDocument, markdownTextForDoc } from "$lib/yjsStore";
  import { getActiveCollab } from "$lib/collab";

  export let docId: string;

  type RemoteCursor = {
    clientId: string;
    position: number;
    color: string;
    name: string;
    selection?: [number, number];
  };

  const setRemoteCursors = StateEffect.define<RemoteCursor[]>();

  class RemoteCaretWidget extends WidgetType {
    private color: string;
    private name: string;

    constructor(color: string, name: string) {
      super();
      this.color = color;
      this.name = name;
    }

    eq(other: RemoteCaretWidget) {
      return this.color === other.color && this.name === other.name;
    }

    toDOM() {
      const caret = document.createElement("span");
      caret.className = "cm-remote-caret";
      caret.style.borderLeftColor = this.color;

      const label = document.createElement("span");
      label.className = "cm-remote-caret-label";
      label.style.backgroundColor = this.color;
      label.textContent = this.name;
      caret.append(label);
      return caret;
    }

    ignoreEvent() {
      return true;
    }
  }

  const remoteCursorField = StateField.define<RemoteCursor[]>({
    create: () => [],
    update(cursors, transaction) {
      for (const effect of transaction.effects) {
        if (effect.is(setRemoteCursors)) return effect.value;
      }
      return cursors;
    },
    provide: (field) =>
      EditorView.decorations.compute([field], (state) => {
        const decorations = [];
        const length = state.doc.length;
        for (const cursor of state.field(field)) {
          const caret = Math.max(0, Math.min(cursor.position, length));
          const [rawStart, rawEnd] = cursor.selection ?? [caret, caret];
          const selectionStart = Math.max(
            0,
            Math.min(Math.min(rawStart, rawEnd), length),
          );
          const selectionEnd = Math.max(
            0,
            Math.min(Math.max(rawStart, rawEnd), length),
          );

          if (selectionStart !== selectionEnd) {
            decorations.push(
              Decoration.mark({
                class: "cm-remote-selection",
                attributes: { style: `background-color: ${cursor.color}33` },
              }).range(selectionStart, selectionEnd),
            );
          }
          decorations.push(
            Decoration.widget({
              widget: new RemoteCaretWidget(cursor.color, cursor.name),
              side: 1,
            }).range(caret),
          );
        }
        return Decoration.set(decorations, true);
      }),
  });

  let editorHost: HTMLDivElement;
  let editorView: EditorView | null = null;
  let ytext = markdownTextForDoc(docId);
  let applyingRemote = false;
  let awarenessUnsubscribe: (() => void) | null = null;
  let observer: ((event: Y.YTextEvent) => void) | null = null;

  function currentRemoteCursors(): RemoteCursor[] {
    return getActiveCollab()?.getDocCursors(docId) ?? [];
  }

  function refreshRemoteCursors() {
    editorView?.dispatch({
      effects: setRemoteCursors.of(currentRemoteCursors()),
    });
  }

  function applyYTextToEditor() {
    if (!editorView) return;
    const next = ytext.toString();
    const current = editorView.state.doc.toString();
    if (next === current) return;

    applyingRemote = true;
    editorView.dispatch({
      changes: { from: 0, to: current.length, insert: next },
    });
    applyingRemote = false;
    refreshRemoteCursors();
  }

  function broadcastLocalAwareness() {
    if (!editorView) return;
    const selection = editorView.state.selection.main;
    getActiveCollab()?.setAwareness({
      anchor: { kind: "docCursor", docId },
      cursor: selection.head,
      selection:
        selection.anchor !== selection.head
          ? [selection.anchor, selection.head]
          : undefined,
    });
  }

  onMount(async () => {
    ensureDefaultDocument(docId);
    ytext = markdownTextForDoc(docId);

    const updateListener = EditorView.updateListener.of((update) => {
      if (update.docChanged && !applyingRemote) {
        const changes = [] as { from: number; to: number; insert: string }[];
        update.changes.iterChanges((fromA, toA, _fromB, _toB, inserted) => {
          changes.push({ from: fromA, to: toA, insert: inserted.toString() });
        });
        ytext.doc?.transact(() => {
          for (let index = changes.length - 1; index >= 0; index--) {
            const change = changes[index];
            ytext.delete(change.from, change.to - change.from);
            if (change.insert) ytext.insert(change.from, change.insert);
          }
        });
      }
      if (update.selectionSet || update.docChanged || update.focusChanged) {
        broadcastLocalAwareness();
      }
    });

    editorView = new EditorView({
      state: EditorState.create({
        doc: ytext.toString(),
        extensions: [
          history(),
          keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
          markdown(),
          syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
          drawSelection(),
          remoteCursorField,
          updateListener,
          EditorView.theme({
            "&": { height: "100%", backgroundColor: "rgba(9, 9, 11, 0.3)" },
            ".cm-scroller": {
              overflow: "auto",
              fontFamily: "'Fira Code VF', monospace",
              lineHeight: "1.75rem",
            },
            ".cm-content": {
              padding: "1.25rem",
              minHeight: "100%",
              caretColor: "#f8fafc",
            },
            ".cm-cursor, .cm-dropCursor": {
              borderLeftColor: "#f8fafc !important",
              borderLeftWidth: "2px !important",
              borderLeftStyle: "solid !important",
            },
            ".cm-focused .cm-cursor": { display: "block !important" },
            ".cm-line": { padding: "0" },
            ".cm-gutters": { display: "none" },
            ".cm-selectionBackground, ::selection": {
              backgroundColor: "rgba(99, 102, 241, 0.35) !important",
            },
            ".cm-remote-caret": {
              position: "relative",
              display: "inline-block",
              height: "1.75rem",
              borderLeft: "2px solid",
              verticalAlign: "bottom",
            },
            ".cm-remote-caret-label": {
              position: "absolute",
              top: "-1rem",
              left: "-2px",
              zIndex: "10",
              borderRadius: "3px",
              padding: "0 4px",
              color: "white",
              fontSize: "10px",
              lineHeight: "14px",
              whiteSpace: "nowrap",
            },
          }),
        ],
      }),
      parent: editorHost,
    });

    observer = () => applyYTextToEditor();
    ytext.observe(observer);
    awarenessUnsubscribe =
      getActiveCollab()?.onAwareness(refreshRemoteCursors) ?? null;
    refreshRemoteCursors();
    await tick();
    editorView.focus();
  });

  onDestroy(() => {
    if (observer) ytext.unobserve(observer);
    observer = null;
    awarenessUnsubscribe?.();
    awarenessUnsubscribe = null;
    getActiveCollab()?.clearDocCursor(docId);
    editorView?.destroy();
    editorView = null;
  });
</script>

<section class="h-full min-h-0 overflow-hidden" data-no-pan>
  <div
    class="h-full min-h-0 overflow-hidden cursor-text"
    bind:this={editorHost}
  ></div>
</section>
