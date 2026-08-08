<template>
  <button
    v-if="hasClipboard"
    :class="{'btn': true, 'btn-primary': !copyToClipboardSuccess, 'btn-success': copyToClipboardSuccess, 'shadow-sm': true}"
    :disabled="!content"
    :title="title || $t('tooltip-copy-to-clipboard') || 'Copy to Clipboard'"
    @click="copy"
  >
    <i :class="{'fas fa-fw fa-clipboard': !copyToClipboardSuccess, 'fas fa-fw fa-check': copyToClipboardSuccess, 'me-1': showLabel}" />
    <span v-if="showLabel" class="small fw-semibold">{{ copyToClipboardSuccess ? 'Copied!' : (labelText || 'Copy') }}</span>
  </button>
</template>

<script lang="ts">
import { defineComponent } from "vue";

export default defineComponent({
	computed: {
		hasClipboard(): boolean {
			return Boolean(navigator.clipboard?.writeText);
		},
	},

	data() {
		return {
			copyToClipboardSuccess: false,
		};
	},

	methods: {
		copy(): void {
			navigator.clipboard.writeText(this.content).then(() => {
				this.copyToClipboardSuccess = true;
				window.setTimeout(() => {
					this.copyToClipboardSuccess = false;
				}, 1500);
			});
		},
	},

	name: "AppClipboardButton",

	props: {
		content: {
			default: null,
			required: false,
			type: String,
		},

		labelText: {
			default: "",
			required: false,
			type: String,
		},

		showLabel: {
			default: false,
			required: false,
			type: Boolean,
		},

		title: {
			default: "",
			required: false,
			type: String,
		},
	},
});
</script>
