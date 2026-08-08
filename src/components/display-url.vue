<template>
  <div :class="burned ? 'card border-danger-subtle shadow-sm mb-3' : 'card border-success-subtle mb-3'">
    <!-- Safe: Trusted internal translation string from i18n.yaml -->
    <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
    <div :class="burned ? 'card-header bg-danger-subtle text-danger-emphasis d-flex justify-content-between align-items-center py-2 fw-bold' : 'card-header bg-success-subtle d-flex justify-content-between align-items-center py-2'">
      <!-- Safe: Trusted internal translation string from i18n.yaml -->
      <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
      <span v-if="!burned" v-html="$t('title-secret-created')" />
      <span v-else class="d-flex align-items-center">
        <i class="fas fa-fire me-2 text-danger" /> Secret Permanently Burned
      </span>
      <app-message-modal-button
        v-if="!burned"
        :secret-url="secretUrl"
        :short-url="shortUrl"
        :secure-password="securePassword"
        :burn-time="burnTime"
        :expires-at="expiresAt"
      />
    </div>
    <div
      v-if="!burned"
      class="card-body"
    >
      <!-- Safe: Trusted internal translation string from i18n.yaml -->
      <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
      <p v-html="$t('text-pre-url')" />
      <div class="input-group mb-3">
        <input
          ref="secretUrl"
          class="form-control"
          type="text"
          readonly
          :value="secretUrl"
          @focus="selectURL"
        >
        <app-clipboard-button
          :content="secretUrl"
          :title="$t('tooltip-copy-full-link')"
        />
        <app-qr-button
          :qr-content="secretUrl"
          :title="$t('tooltip-qr-code')"
        />
        <button
          class="btn btn-danger"
          :title="$t('tooltip-burn-secret')"
          @click="showBurnModal = true"
        >
          <i class="fas fa-fire fa-fw" />
        </button>
      </div>

      <!-- Burn Confirmation Modal Popup (Sender) -->
      <app-burn-modal
        :show="showBurnModal"
        :secret-id="secretId"
        @close="showBurnModal = false"
        @confirm="confirmBurnSecret"
      />

      <!-- Dual Channel Link Section -->
      <div class="card bg-body-tertiary mb-3">
        <div class="card-body p-3">
          <h6 class="card-subtitle mb-2 text-body-secondary"><i class="fas fa-shield-halved me-1" /> Dual-Channel Delivery (Key Separated)</h6>
          <div class="row g-2">
            <div class="col-md-7">
              <label class="form-label small mb-1">Short Link (without key):</label>
              <div class="input-group input-group-sm">
                <input class="form-control" type="text" readonly :value="shortUrl">
                <app-clipboard-button :content="shortUrl" :title="$t('tooltip-copy-short-link')" />
              </div>
            </div>
            <div class="col-md-5">
              <label class="form-label small mb-1">Decryption Key:</label>
              <div class="input-group input-group-sm">
                <input class="form-control" type="text" readonly :value="securePassword">
                <app-clipboard-button :content="securePassword" :title="$t('tooltip-copy-decryption-key')" />
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Safe: Trusted internal translation string from i18n.yaml -->
      <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
      <p v-if="expiresAt" class="d-flex flex-wrap align-items-center gap-2 mb-0">
        <span>{{ $t('text-burn-time') }} <strong>{{ expiresAt.toLocaleString() }}</strong></span>
        <span v-if="countdownText" class="badge bg-warning-subtle text-warning-emphasis border border-warning-subtle font-monospace px-2 py-1">
          <i class="fas fa-clock me-1" /> Expires in {{ countdownText }}
        </span>
      </p>
    </div>
    <app-burned-display v-else />
  </div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import appBurnModal from "./burn-modal.vue";
import appBurnedDisplay from "./burned-display.vue";
import appClipboardButton from "./clipboard-button.vue";
import appMessageModalButton from "./message-modal.vue";
import appQrButton from "./qr-button.vue";

export default defineComponent({
	components: { appBurnModal, appBurnedDisplay, appClipboardButton, appMessageModalButton, appQrButton },

	computed: {
		burnTime(): string {
			if (!this.expiresAt) return "";
			try {
				return this.expiresAt.toLocaleString(undefined, {
					year: "numeric",
					month: "numeric",
					day: "numeric",
					hour: "numeric",
					minute: "2-digit",
					second: "2-digit",
					timeZoneName: "short",
				});
			} catch (_e) {
				return this.expiresAt.toLocaleString();
			}
		},

		secretUrl(): string {
			return [
				window.location.href.split("#")[0],
				encodeURIComponent([this.secretId, this.securePassword].join("|")),
			].join("#");
		},

		shortUrl(): string {
			return [
				window.location.href.split("#")[0],
				encodeURIComponent(this.secretId),
			].join("#");
		},
	},

	data() {
		return {
			burned: false,
			countdownText: "",
			showBurnModal: false,
			timerId: null as number | null,
		};
	},

	methods: {
		confirmBurnSecret(): Promise<void> {
			return fetch(`/api/burn/${this.secretId}`, { method: "POST" })
				.then((resp) => {
					if (resp.ok) {
						this.burned = true;
						this.showBurnModal = false;
					}
				});
		},

		selectURL(): void {
			this.$refs.secretUrl.select();
		},

		updateCountdown(): void {
			if (!this.expiresAt) {
				this.countdownText = "";
				return;
			}
			const diffMs = this.expiresAt.getTime() - Date.now();
			if (diffMs <= 0) {
				this.countdownText = "Expired";
				return;
			}
			const totalSec = Math.floor(diffMs / 1000);
			const h = Math.floor(totalSec / 3600);
			const m = Math.floor((totalSec % 3600) / 60);
			const s = totalSec % 60;
			if (h > 0) {
				this.countdownText = `${h}h ${m}m ${s}s`;
			} else if (m > 0) {
				this.countdownText = `${m}m ${s}s`;
			} else {
				this.countdownText = `${s}s`;
			}
		},
	},

	mounted(): void {
		// Give the interface a moment to transistion and focus
		window.setTimeout(() => this.$refs.secretUrl.focus(), 100);
		this.updateCountdown();
		this.timerId = window.setInterval(() => this.updateCountdown(), 1000);
	},

	unmounted(): void {
		if (this.timerId) {
			window.clearInterval(this.timerId);
		}
	},

	name: "AppDisplayURL",

	props: {
		expiresAt: {
			default: null,
			required: false,
			type: Date,
		},

		secretId: {
			required: true,
			type: String,
		},

		securePassword: {
			required: true,
			type: String,
		},
	},
});
</script>
