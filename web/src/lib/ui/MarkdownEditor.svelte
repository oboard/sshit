<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import DOMPurify from "dompurify";
  import { marked } from "marked";
  import {
    BoldIcon,
    CodeIcon,
    EyeIcon,
    HashIcon,
    ItalicIcon,
    ListIcon,
    TypeIcon,
  } from "svelte-feather-icons";
  import * as Y from "yjs";

  import { ensureDefaultDocument, markdownTextForDoc } from "$lib/yjsStore";
  import { getActiveCollab } from "$lib/collab";
  import { settings } from "$lib/settings";

  export let docId: string;
  export let activeUsers = 1;
  export let synced = false;
  export let compact = false;
  export let userId = 0;

  let textarea: HTMLTextAreaElement;
  let value = "";
  let html = "";
  let applyingRemote = false;
  let splitView = true;
  let ytext = markdownTextForDoc(docId);
  let remoteCursors: { position: number; color: string; name: string; selection?: [number, number] }[] = [];
  let awarenessUnsubscribe: (() => void) | null = null;

  marked.use({
    gfm: true,
    breaks: true,
  });

  $: html = DOMPurify.sanitize(marked.parse(value, { async: false }) as string);

  function syncFromYText() {
    applyingRemote = true;
    value = ytext.toString();
    requestAnimationFrame(() => {
      applyingRemote = false;
      refreshRemoteCursors();
    });
  }

  function replaceWholeDocument(nextValue: string) {
    ytext.doc?.transact(() => {
      ytext.delete(0, ytext.length);
      ytext.insert(0, nextValue);
    });
  }

  function handleInput() {
    if (applyingRemote) return;
    replaceWholeDocument(value);
    broadcastLocalCursor();
  }

  function broadcastLocalCursor() {
    const pos = textarea?.selectionStart ?? 0;
    const selEnd = textarea?.selectionEnd ?? pos;
    getActiveCollab()?.setAwareness({
      anchor: { kind: "docCursor", docId },
      cursor: pos,
      selection: pos !== selEnd ? [pos, selEnd] : undefined,
      name: $settings.name || `user-${userId}`,
      color: "#f472b6",
    });
  }

  function wrapSelection(prefix: string, suffix = prefix, placeholder = "文本") {
    textarea?.focus();
    const start = textarea?.selectionStart ?? value.length;
    const end = textarea?.selectionEnd ?? value.length;
    const selected = value.slice(start, end) || placeholder;
    const nextValue = value.slice(0, start) + prefix + selected + suffix + value.slice(end);
    value = nextValue;
    replaceWholeDocument(nextValue);
    requestAnimationFrame(() => {
      textarea?.setSelectionRange(start + prefix.length, start + prefix.length + selected.length);
      broadcastLocalCursor();
    });
  }

  function prefixLines(prefix: string) {
    textarea?.focus();
    const start = textarea?.selectionStart ?? value.length;
    const end = textarea?.selectionEnd ?? value.length;
    const lineStart = value.lastIndexOf("\n", Math.max(0, start - 1)) + 1;
    const selected = value.slice(lineStart, end) || "新内容";
    const nextSelected = selected
      .split("\n")
      .map((line) => (line.startsWith(prefix) ? line : `${prefix}${line}`))
      .join("\n");
    const nextValue = value.slice(0, lineStart) + nextSelected + value.slice(end);
    value = nextValue;
    replaceWholeDocument(nextValue);
    requestAnimationFrame(() => broadcastLocalCursor());
  }

  function refreshRemoteCursors() {
    remoteCursors = getActiveCollab()?.getDocCursors(docId) ?? [];
  }

  function escapeHtml(str: string) {
    return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  }

  function renderSourceWithCursors(rawText: string) {
    if (!remoteCursors.length) return escapeHtml(rawText);

    // Insert cursor marks into text — we need to work with offset positions
    let htmlText = "";
    let lastOffset = 0;

    // Build sorted list of cursor highlights (one per line)
    // We mark whole lines for each cursor so the last one wins visually
    const sorted = [...remoteCursors].sort((a, b) => a.position - b.position);

    for (const cursor of sorted) {
      if (cursor.position < 0 || cursor.position >= rawText.length) continue;
      const lineStart = rawText.lastIndexOf("\n", Math.max(0, cursor.position - 1)) + 1;
      const lineEnd = rawText.indexOf("\n", cursor.position) !== -1 ? rawText.indexOf("\n", cursor.position) : rawText.length;

      htmlText += escapeHtml(rawText.slice(lastOffset, lineStart));
      htmlText += `<span class="remote-cursor-line" style="border-left: 2px solid ${cursor.color}; background: ${cursor.color}10; position: relative;" data-cursor-name="${escapeHtml(cursor.name)}" data-cursor-color="${cursor.color}" data-cursor-pos="${cursor.position}">`;
      htmlText += escapeHtml(rawText.slice(lineStart, lineEnd));
      htmlText += `<span class="cursor-label" style="position: absolute; right: 0; top: -1.2em; background: ${cursor.color}; color: #fff; font-size: 10px; padding: 0 4px; border-radius: 3px; white-space: nowrap; pointer-events: none;">${escapeHtml(cursor.name)}</span></span>`;
      htmlText += escapeHtml(rawText.slice(lineEnd, Math.min(lineEnd + 200, rawText.length)));
      lastOffset = lineEnd + 1;
    }

    htmlText += escapeHtml(rawText.slice(lastOffset));
    return htmlText;
  }

  onMount(() => {
    ensureDefaultDocument(docId);
    ytext = markdownTextForDoc(docId);
    syncFromYText();
    ytext.observe(syncFromYText);
    textarea?.addEventListener("focus", broadcastLocalCursor);
    textarea?.addEventListener("blur", () => getActiveCollab()?.clearDocCursor(docId));
    textarea?.addEventListener("pointerup", broadcastLocalCursor);
    textarea?.addEventListener("keyup", broadcastLocalCursor);
    if (getActiveCollab()) {
      awarenessUnsubscribe = getActiveCollab()!.onAwareness(() => {
        refreshRemoteCursors();
      });
    }
  });

  onDestroy(() => {
    ytext.unobserve(syncFromYText as (event: Y.YTextEvent) => void);
    textarea?.removeEventListener("focus", broadcastLocalCursor);
    textarea?.removeEventListener("blur", () => getActiveCollab()?.clearDocCursor(docId));
    textarea?.removeEventListener("pointerup", broadcastLocalCursor);
    textarea?.removeEventListener("keyup", broadcastLocalCursor);
    getActiveCollab()?.clearDocCursor(docId);
    awarenessUnsubscribe?.();
    awarenessUnsubscribe = null;
  });
</script>

<section class="panel editor-shell" class:compact data-no-pan>
  {#if !compact}
    <div class="editor-header">
      <div>
        <p class="eyebrow">Yjs Markdown</p>
        <h2>协作所见即所得编辑器</h2>
      </div>
      <div class="flex items-center gap-2 text-xs text-zinc-400">
        <span class="rounded-full border px-2 py-1" class:connected-pill={synced}>
          {activeUsers} online · {synced ? "synced" : "local"}
        </span>
        <button class="toolbar-button" class:active={splitView} on:click={() => (splitView = !splitView)} title="切换预览">
          <EyeIcon strokeWidth={1.5} class="h-4 w-4" />
        </button>
      </div>
    </div>
  {/if}

  <div class="format-bar">
    <button class="format-button" on:click={() => prefixLines("# ")} title="标题">
      <HashIcon strokeWidth={1.5} class="h-4 w-4" /> 标题
    </button>
    <button class="format-button" on:click={() => wrapSelection("**", "**", "加粗文本")} title="加粗">
      <BoldIcon strokeWidth={1.5} class="h-4 w-4" /> 加粗
    </button>
    <button class="format-button" on:click={() => wrapSelection("*", "*", "斜体文本")} title="斜体">
      <ItalicIcon strokeWidth={1.5} class="h-4 w-4" /> 斜体
    </button>
    <button class="format-button" on:click={() => prefixLines("- ")} title="列表">
      <ListIcon strokeWidth={1.5} class="h-4 w-4" /> 列表
    </button>
    <button class="format-button" on:click={() => wrapSelection("`", "`", "code")} title="代码">
      <CodeIcon strokeWidth={1.5} class="h-4 w-4" /> 代码
    </button>
    <button class="format-button" on:click={() => prefixLines("> ")} title="引用">
      <TypeIcon strokeWidth={1.5} class="h-4 w-4" /> 引用
    </button>
  </div>

  <div class="editor-grid" class:single={!splitView}>
    <div class="source-editor relative">
      <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
      <div
        class="collab-gutter"
        bind:this={textarea}
        tabindex="0"
        contenteditable="true"
        spellcheck="false"
        role="textbox"
        aria-label="Collaborative Markdown source"
        aria-multiline="true"
        on:input={handleInput}
        on:focus={broadcastLocalCursor}
        on:blur={() => getActiveCollab()?.clearDocCursor(docId)}
        on:pointerup={broadcastLocalCursor}
        on:keyup={broadcastLocalCursor}
      >
        {@html renderSourceWithCursors(value)}
      </div>
    </div>
    {#if splitView}
      <article class="preview wysiwyg" aria-label="Markdown preview">
        {@html html}
      </article>
    {/if}
  </div>
</section>

<style lang="postcss">
  .editor-shell {
    @apply flex h-full min-h-[520px] flex-col overflow-hidden shadow-2xl shadow-black/30;
  }

  .editor-shell.compact {
    @apply min-h-0 rounded-none border-0 bg-transparent shadow-none;
  }

  .editor-header {
    @apply flex items-center justify-between border-b border-zinc-800 px-5 py-4;
  }

  .editor-header h2 {
    @apply text-lg font-semibold tracking-tight text-zinc-100;
  }

  .eyebrow {
    @apply mb-1 text-xs uppercase tracking-[0.2em] text-pink-300/80;
  }

  .connected-pill {
    @apply border-emerald-500/30 bg-emerald-500/10 text-emerald-200;
  }

  .toolbar-button,
  .format-button {
    @apply inline-flex items-center gap-1.5 rounded-md border border-zinc-800 bg-zinc-950/60 px-2.5 py-1.5 text-sm text-zinc-300 transition-colors hover:bg-zinc-800 hover:text-white active:bg-indigo-700;
  }

  .toolbar-button.active,
  .format-button:focus-visible {
    @apply border-indigo-500/60 bg-indigo-600/20 outline-none;
  }

  .format-bar {
    @apply flex flex-wrap gap-2 border-b border-zinc-800 bg-zinc-950/40 px-5 py-3;
  }

  .compact .format-bar {
    @apply px-3 py-2;
  }

  .compact .format-button {
    @apply px-2 py-1 text-xs;
  }

  .editor-grid {
    @apply grid min-h-0 flex-1 grid-cols-2;
  }

  .editor-grid.single {
    @apply grid-cols-1;
  }

  .source-editor {
    @apply relative;
  }

  .collab-gutter {
    @apply h-full w-full overflow-auto border-r border-zinc-800 bg-zinc-950/30 p-5 font-mono text-[15px] leading-7 text-zinc-100 outline-none whitespace-pre-wrap;
  }

  .preview {
    @apply h-full overflow-auto bg-zinc-900/40 p-6 text-zinc-100;
  }

  .wysiwyg :global(h1) {
    @apply mb-4 border-b border-zinc-700 pb-3 text-3xl font-semibold tracking-tight;
  }

  .wysiwyg :global(h2) {
    @apply mb-3 mt-6 text-2xl font-semibold;
  }

  .wysiwyg :global(h3) {
    @apply mb-2 mt-5 text-xl font-semibold;
  }

  .wysiwyg :global(p) {
    @apply my-3 leading-7 text-zinc-200;
  }

  .wysiwyg :global(ul),
  .wysiwyg :global(ol) {
    @apply my-3 ml-6 list-outside leading-7 text-zinc-200;
  }

  .wysiwyg :global(ul) {
    @apply list-disc;
  }

  .wysiwyg :global(ol) {
    @apply list-decimal;
  }

  .wysiwyg :global(blockquote) {
    @apply my-4 border-l-4 border-pink-500/70 bg-pink-500/10 px-4 py-2 text-zinc-200;
  }

  .wysiwyg :global(code) {
    @apply rounded bg-black/40 px-1.5 py-0.5 font-mono text-sm text-pink-200;
  }

  .wysiwyg :global(pre) {
    @apply my-4 overflow-auto rounded-lg border border-zinc-800 bg-black/40 p-4;
  }

  .wysiwyg :global(pre code) {
    @apply bg-transparent p-0 text-zinc-100;
  }

  .wysiwyg :global(a) {
    @apply text-indigo-300 underline decoration-indigo-400/50 underline-offset-4;
  }

  @media (max-width: 900px) {
    .editor-grid {
      @apply grid-cols-1;
    }

    .collab-gutter {
      @apply border-r-0 border-b;
      min-height: 260px;
    }
  }
</style>