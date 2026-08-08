<template>
  <span v-if="!customize.disableQRSupport">
    <button
      ref="qrButton"
      class="btn btn-secondary shadow-sm"
      :disabled="!qrDataURL"
      :title="computedTitle"
      @click="showModal = true"
    >
      <i class="fas fa-qrcode" />
    </button>

    <!-- Mobile-Friendly High-Contrast QR Code Modal -->
    <div
      v-if="showModal"
      class="modal fade show d-block"
      tabindex="-1"
      style="background-color: rgba(0, 0, 0, 0.6);"
      @click.self="showModal = false"
    >
      <div class="modal-dialog modal-dialog-centered modal-sm">
        <div class="modal-content shadow-lg border-primary-subtle">
          <div class="modal-header bg-primary-subtle py-2">
            <h6 class="modal-title fw-bold text-primary-emphasis mb-0">
              <i class="fas fa-qrcode me-2" /> {{ computedTitle }}
            </h6>
            <button
              type="button"
              class="btn-close"
              @click="showModal = false"
            />
          </div>
          <div class="modal-body text-center p-4">
            <div class="p-3 bg-white rounded border d-inline-block shadow-sm mb-3">
              <img
                v-if="qrDataURL"
                :src="qrDataURL"
                alt="QR Code"
                class="img-fluid"
                style="max-width: 220px;"
              >
            </div>
            <p class="small text-secondary mb-0">
              Scan with your smartphone camera to access link directly.
            </p>
          </div>
          <div class="modal-footer py-2 justify-content-center">
            <button
              type="button"
              class="btn btn-sm btn-secondary"
              @click="showModal = false"
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </div>
  </span>
</template>

<script lang="ts">
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
			showModal: false,
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
	},
});
</script>
