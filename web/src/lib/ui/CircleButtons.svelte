<div class="flex space-x-2 text-transparent hover:text-black/75 circle-buttons">
  <slot />
</div>

<style lang="postcss">
  /* There is no hover on touch screens — always show the button glyphs. */
  @media (hover: none) {
    .circle-buttons {
      color: rgb(0 0 0 / 0.6);
    }
  }
</style>
