<script lang="ts">
  import { ChevronDownIcon } from "svelte-feather-icons";

  import { settings, updateSettings } from "$lib/settings";
  import OverlayMenu from "./OverlayMenu.svelte";
  import themes, { type ThemeName } from "./themes";

  export let open: boolean;

  let inputName: string;
  let inputTheme: ThemeName;
  let inputScrollback: number;

  let initialized = false;
  $: open, (initialized = false);
  $: if (!initialized) {
    initialized = true;
    inputName = $settings.name;
    inputTheme = $settings.theme;
    inputScrollback = $settings.scrollback;
  }
</script>

<OverlayMenu
  title="Terminal Settings"
  description="Customize your collaborative terminal."
  showCloseButton
  {open}
  on:close
>
  <div class="flex flex-col gap-4">
    <div class="flex flex-col items-start gap-4 rounded-lg bg-zinc-800/25 p-4 sm:flex-row">
      <div class="flex-1">
        <p class="mb-1 font-medium text-zinc-200">Name</p>
        <p class="text-sm text-zinc-400">Choose how you appear to other users.</p>
      </div>
      <div>
        <input
          class="w-52 appearance-none rounded-md border border-zinc-700 bg-transparent px-3 py-2 text-sm outline-none transition-colors hover:bg-white/5 focus:ring-2 focus:ring-indigo-500/50"
          placeholder="Your name"
          bind:value={inputName}
          maxlength="50"
          on:input={() => {
            if (inputName.length >= 2) {
              updateSettings({ name: inputName });
            }
          }}
        />
      </div>
    </div>
    <div class="flex flex-col items-start gap-4 rounded-lg bg-zinc-800/25 p-4 sm:flex-row">
      <div class="flex-1">
        <p class="mb-1 font-medium text-zinc-200">Color palette</p>
        <p class="text-sm text-zinc-400">Color theme for text in terminals.</p>
      </div>
      <div class="relative">
        <ChevronDownIcon
          class="absolute top-[11px] right-2.5 w-4 h-4 text-zinc-400"
        />
        <select
          class="w-52 appearance-none rounded-md border border-zinc-700 bg-transparent px-3 py-2 pr-5 text-sm outline-none transition-colors hover:bg-white/5 focus:ring-2 focus:ring-indigo-500/50"
          bind:value={inputTheme}
          on:change={() => updateSettings({ theme: inputTheme })}
        >
          {#each Object.keys(themes) as themeName (themeName)}
            <option value={themeName}>{themeName}</option>
          {/each}
        </select>
      </div>
    </div>
    <div class="flex flex-col items-start gap-4 rounded-lg bg-zinc-800/25 p-4 sm:flex-row">
      <div class="flex-1">
        <p class="mb-1 font-medium text-zinc-200">Scrollback</p>
        <p class="text-sm text-zinc-400">
          Lines of previous text displayed in the terminal window.
        </p>
      </div>
      <div>
        <input
          type="number"
          class="w-52 appearance-none rounded-md border border-zinc-700 bg-transparent px-3 py-2 text-sm outline-none transition-colors hover:bg-white/5 focus:ring-2 focus:ring-indigo-500/50"
          bind:value={inputScrollback}
          on:input={() => {
            if (inputScrollback >= 0) {
              updateSettings({ scrollback: inputScrollback });
            }
          }}
          step="100"
        />
      </div>
    </div>
    <!-- <div class="item">
      <div>
        <p class="item-title">Cursor style</p>
        <p class="item-subtitle">Style of live cursors.</p>
      </div>
      <div class="text-red-500">Coming soon</div>
    </div> -->
  </div>

  <p class="mt-6 text-sm text-right text-zinc-400">sshit</p>
</OverlayMenu>
