<!-- eslint-disable vue/no-v-html -->
<template>
  <div class="card border-success-subtle mb-3">
    <!-- Safe: Trusted internal translation string from i18n.yaml -->
    <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
    <div class="card-header bg-success-subtle d-flex justify-content-between align-items-center py-2">
      <!-- Safe: Trusted internal translation string from i18n.yaml -->
      <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
      <span v-html="$t('title-secret-created')" />
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
          @click="burnSecret"
        >
          <i class="fas fa-fire fa-fw" />
        </button>
      </div>

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
      <p v-html="$t('text-burn-hint')" />
      <p v-if="expiresAt">
        {{ $t('text-burn-time') }}
        <strong>{{ expiresAt.toLocaleString() }}</strong>
      </p>
    </div>
    <div
      v-else
      class="card-body"
    >
      {{ $t('text-secret-burned') }}
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import appClipboardButton from "./clipboard-button.vue";
import appMessageModalButton from "./message-modal.vue";
import appQrButton from "./qr-button.vue";

export default defineComponent({
	components: { appClipboardButton, appMessageModalButton, appQrButton },

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
			popover: null,
		};
	},

	methods: {
		burnSecret(): Promise<void> {
			return fetch(`/api/get/${this.secretId}`).then(() => {
				this.burned = true;
			});
		},

		selectURL(): void {
			this.$refs.secretUrl.select();
		},
	},

	mounted(): void {
		// Give the interface a moment to transistion and focus
		window.setTimeout(() => this.$refs.secretUrl.focus(), 100);
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
