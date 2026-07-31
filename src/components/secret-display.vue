<template>
  <div class="card border-primary-subtle mb-3">
    <!-- Safe: Trusted internal translation string from i18n.yaml -->
    <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
    <div
      class="card-header bg-primary-subtle"
      v-html="$t('title-reading-secret')"
    />
    <div class="card-body">
      <template v-if="!secret && files.length === 0">
        <!-- Safe: Trusted internal translation string from i18n.yaml -->
        <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
        <p v-html="$t('text-pre-reveal-hint')" />
        <div v-if="!securePassword" class="mb-3">
          <label for="decryptionKeyInput" class="form-label">Enter Decryption Key:</label>
          <input
            id="decryptionKeyInput"
            v-model="inputPassword"
            type="text"
            class="form-control"
            placeholder="Paste or enter decryption key..."
          >
        </div>
        <button
          class="btn btn-success"
          :disabled="secretLoading || (!securePassword && !inputPassword)"
          @click="requestSecret"
        >
          <template v-if="!secretLoading">
            {{ $t('btn-reveal-secret') }}
          </template>
          <template v-else>
            <i class="fa-solid fa-spinner fa-spin-pulse" />
            {{ $t('btn-reveal-secret-processing') }}
          </template>
        </button>
      </template>
      <template v-else>
        <div
          v-if="secret"
          class="input-group mb-3"
        >
          <grow-area
            class="form-control"
            readonly
            :value="secret"
            :rows="4"
          />
          <div class="d-flex align-items-start p-0">
            <div
              class="btn-group-vertical"
              role="group"
            >
              <app-clipboard-button
                :content="secret"
                :title="$t('tooltip-copy-secret-content')"
              />
              <a
                class="btn btn-secondary"
                :href="secretContentBlobURL || ''"
                download
                :title="$t('tooltip-download-as-file')"
              >
                <i class="fas fa-fw fa-download" />
              </a>
              <app-qr-button :qr-content="secret" />
            </div>
          </div>
        </div>
        <template v-if="files.length > 0">
          <!-- Safe: Trusted internal translation string from i18n.yaml -->
          <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
          <p v-html="$t('text-attached-files')" />
          <FilesDisplay :files="files" />
        </template>
        <!-- Safe: Trusted internal translation string from i18n.yaml -->
        <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
        <p v-html="$t('text-hint-burned')" />
      </template>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import appCrypto from "../crypto.ts";
import OTSMeta from "../ots-meta";
import appClipboardButton from "./clipboard-button.vue";
import FilesDisplay from "./fileDisplay.vue";
import GrowArea from "./growarea.vue";
import appQrButton from "./qr-button.vue";

export default defineComponent({
	components: { FilesDisplay, GrowArea, appClipboardButton, appQrButton },

	data() {
		return {
			files: [],
			inputPassword: "",
			popover: null,
			secret: null,
			secretContentBlobURL: null,
			secretLoading: false,
		};
	},

	emits: ["error"],

	methods: {
		// requestSecret requests the encrypted secret from the backend
		requestSecret(): void {
			this.secretLoading = true;
			const keyToUse = this.securePassword || this.inputPassword;
			window.history.replaceState({}, "", window.location.href.split("#")[0]);
			fetch(`/api/get/${this.secretId}`)
				.then((resp) => {
					if (resp.status === 404) {
						// Secret has already been consumed
						this.$emit("error", this.$t("alert-secret-not-found"));
						return;
					}

					if (resp.status !== 200) {
						// Some other non-200: Something(tm) was wrong
						this.$emit("error", this.$t("alert-something-went-wrong"));
						return;
					}

					resp.json().then((data) => {
						const secret = data.secret;
						if (!keyToUse) {
							this.secret = secret;
							return;
						}

						appCrypto
							.dec(secret, keyToUse)
							.then((secret) => {
								const meta = new OTSMeta(secret);
								this.secret = meta.secret;

								meta.files.forEach((file) => {
									file.arrayBuffer().then((ab) => {
										const blobURL = window.URL.createObjectURL(
											new Blob([ab], { type: file.type }),
										);
										this.files.push({
											id: window.crypto.randomUUID(),
											name: file.name,
											size: ab.byteLength,
											type: file.type,
											url: blobURL,
										});
									});
								});
								this.secretLoading = false;
							})
							.catch(() =>
								this.$emit("error", this.$t("alert-something-went-wrong")),
							);
					});
				})
				.catch(() => {
					// Network error
					this.$emit("error", this.$t("alert-something-went-wrong"));
				});
		},
	},

	name: "AppSecretDisplay",
	props: {
		secretId: {
			required: true,
			type: String,
		},

		securePassword: {
			default: null,
			required: false,
			type: String,
		},
	},

	watch: {
		secret(to) {
			if (this.secretContentBlobURL) {
				window.URL.revokeObjectURL(this.secretContentBlobURL);
			}
			this.secretContentBlobURL = window.URL.createObjectURL(
				new Blob([to], { type: "text/plain" }),
			);
		},
	},
});
</script>
