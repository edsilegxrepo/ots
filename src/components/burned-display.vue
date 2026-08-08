<!--
  AppBurnedDisplay Component

  Objectives:
  - Renders a normalized, high-contrast visual display when a secret has been permanently burned.
  - Used by both sender (display-url.vue) and receiver (secret-display.vue) views for 100% DRY template consistency.

  Core Components:
  - Centered flame icon (fa-fire-flame-curved), title, description, and [ + Create Another Secret ] action button.
-->
<template>
  <div class="card-body text-center p-4">
    <div class="mb-3">
      <i class="fas fa-fire-flame-curved fa-3x text-danger opacity-75" />
    </div>
    <h5 class="fw-bold text-danger-emphasis mb-2">Secret Permanently Destroyed</h5>
    <p class="text-secondary mb-3">
      This secret and all associated access links have been permanently erased from server memory.
    </p>
    <a href="/" class="btn btn-sm btn-outline-primary px-3 shadow-sm">
      <i class="fas fa-plus me-1" /> Create Another Secret
    </a>
  </div>
</template>

<script lang="ts">
import { defineComponent } from "vue";

export default defineComponent({
	name: "AppBurnedDisplay",
});
</script>
