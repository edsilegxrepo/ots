<template>
  <button
    v-if="!customize.disableQRSupport"
    id="secret-url-qrcode"
    ref="qrButton"
    class="btn btn-secondary"
    :disabled="!qrDataURL"
    :title="computedTitle"
    :data-bs-title="computedTitle"
    @mouseenter="fixTooltip"
  >
    <i class="fas fa-qrcode" />
  </button>
</template>

<script lang="ts">
import { Popover } from "bootstrap";
import qrcode from "qrcode";
import { defineComponent } from "vue";

export default defineComponent({
	computed: {
		computedTitle(): string {
			return (
				this.title ||
				(this.$t("tooltip-qr-code") as string) ||
				"QR Code for secret URL"
			);
		},

		customize(): any {
			return window.OTSCustomize || {};
		},
	},

	data() {
		return {
			qrDataURL: null,
		};
	},

	methods: {
		fixTooltip(): void {
			if (this.$refs.qrButton) {
				(this.$refs.qrButton as HTMLElement).setAttribute(
					"title",
					this.computedTitle,
				);
			}
		},

		generateQR(): void {
			if (window.OTSCustomize.disableQRSupport) {
				return;
			}

			qrcode.toDataURL(this.qrContent, { width: 200 }).then((url) => {
				this.qrDataURL = url;
			});
		},
	},

	mounted(): void {
		this.generateQR();
	},

	name: "AppQRButton",

	props: {
		qrContent: {
			required: true,
			type: String,
		},

		title: {
			default: "",
			required: false,
			type: String,
		},
	},

	watch: {
		qrContent() {
			this.generateQR();
		},

		qrDataURL(to: string): void {
			if (this.popover) {
				this.popover.dispose();
			}

			this.popover = new Popover(this.$refs.qrButton, {
				content: () => {
					const img = document.createElement("img");
					img.src = to;
					return img;
				},

				html: true,
				placement: "left",
				trigger: "focus",
			});

			if (this.$refs.qrButton) {
				(this.$refs.qrButton as HTMLElement).setAttribute(
					"title",
					this.computedTitle,
				);
			}
		},
	},
});
</script>
