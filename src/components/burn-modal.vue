<!--
  AppBurnModal Component

  Objectives:
  - Renders a centered Bootstrap Modal popup to confirm early permanent secret destruction.
  - Displays Secret ID, remaining view count (if > 0), and explicit warning details.
  - Used by both sender (display-url.vue) and receiver (secret-display.vue) components.

  Props:
  - show: Controls modal visibility backdrop.
  - secretId: Target secret UUID.
  - readsRemaining: Optional allowed view count.
  - isBurning: Spinner state flag during async HTTP POST /api/burn/{id} request.
-->
<template>
  <div
    v-if="show"
    class="modal fade show d-block"
    tabindex="-1"
    style="background-color: rgba(0, 0, 0, 0.6);"
    @click.self="$emit('close')"
  >
    <div class="modal-dialog modal-dialog-centered">
      <div class="modal-content shadow-lg border-danger-subtle">
        <div class="modal-header bg-danger-subtle py-2">
          <h6 class="modal-title fw-bold text-danger-emphasis mb-0 d-flex align-items-center">
            <i class="fas fa-fire text-danger me-2" /> Confirm Permanent Burn
          </h6>
          <button
            type="button"
            class="btn-close"
            :disabled="isBurning"
            @click="$emit('close')"
          />
        </div>
        <div class="modal-body p-4 text-center">
          <div class="mb-3">
            <i class="fas fa-triangle-exclamation fa-3x text-danger opacity-75" />
          </div>
          <h5 class="fw-bold mb-2">Permanently Destroy Secret?</h5>
          <p class="text-secondary mb-2">
            Secret ID: <code class="bg-body-tertiary px-2 py-1 rounded border text-danger-emphasis font-monospace">{{ secretId ? secretId.substring(0, 8) : 'payload' }}</code>
          </p>
          <p v-if="readsRemaining !== undefined && readsRemaining > 0" class="text-secondary small mb-2">
            This secret has <strong>{{ readsRemaining }} allowed view(s) remaining</strong>.
          </p>
          <p class="small text-danger-emphasis bg-danger-subtle p-2 rounded border border-danger-subtle mb-0">
            <i class="fas fa-circle-info me-1" />
            Access to this secret will be immediately revoked and permanently deleted from server memory.
          </p>
        </div>
        <div class="modal-footer py-2 justify-content-center bg-body-tertiary gap-2">
          <button
            type="button"
            class="btn btn-sm btn-secondary"
            :disabled="isBurning"
            @click="$emit('close')"
          >
            Cancel
          </button>
          <button
            type="button"
            class="btn btn-sm btn-danger shadow-sm px-3"
            :disabled="isBurning"
            @click="$emit('confirm')"
          >
            <i :class="isBurning ? 'fas fa-spinner fa-spin me-1' : 'fas fa-fire me-1'" />
            {{ isBurning ? 'Burning...' : 'Permanently Burn Secret' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent } from "vue";

export default defineComponent({
	name: "AppBurnModal",

	props: {
		isBurning: {
			default: false,
			type: Boolean,
		},
		readsRemaining: {
			default: undefined,
			type: Number,
		},
		secretId: {
			default: "",
			type: String,
		},
		show: {
			default: false,
			type: Boolean,
		},
	},

	emits: ["close", "confirm"],
});
</script>
