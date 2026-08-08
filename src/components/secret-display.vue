<template>
  <div class="card border-primary-subtle mb-3">
    <div class="card-header bg-primary-subtle d-flex justify-content-between align-items-center">
      <div class="d-flex align-items-center">
        <!-- Safe: Trusted internal translation string from i18n.yaml -->
        <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
        <span v-html="$t('title-reading-secret')" />
        <span
          v-if="senderNote"
          class="badge bg-primary-subtle text-primary border border-primary-subtle px-2 py-1 ms-2"
        >
          <i class="fas fa-envelope me-1" />
          {{ $t('text-message-received') }}
        </span>
      </div>
      <div v-if="secret || files.length > 0" class="d-flex gap-2">
        <button
          class="btn btn-sm btn-outline-primary shadow-sm"
          :disabled="isGeneratingBundle"
          @click="downloadBundle"
        >
          <i :class="isGeneratingBundle ? 'fas fa-spinner fa-spin me-1' : 'fas fa-file-archive me-1'" />
          {{ isGeneratingBundle ? $t('btn-downloading-bundle') : $t('btn-download-bundle') }}
        </button>
      </div>
    </div>
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
        <!-- Sender Note Display Container -->
        <div v-if="senderNote" class="card border-info-subtle mb-3 shadow-sm">
          <div class="card-header bg-info-subtle d-flex justify-content-between align-items-center py-2">
            <span class="fw-semibold text-info-emphasis">
              <i class="fas fa-envelope-open-text me-1" /> {{ $t('text-message-title') }}
            </span>
            <div class="d-flex align-items-center gap-2">
              <span class="badge bg-info text-dark rounded-pill px-3 py-2 font-monospace">{{ senderMessageFormat }}</span>
              <button
                v-if="senderMessageFormat === 'html' || senderMessageFormat === 'md'"
                type="button"
                class="btn btn-sm btn-outline-info text-dark shadow-sm py-0 px-2"
                style="font-size: 0.75rem;"
                @click="showRawNote = !showRawNote"
              >
                <i :class="showRawNote ? 'fas fa-eye me-1' : 'fas fa-code me-1'" />
                {{ showRawNote ? 'Rendered View' : 'Raw Source' }}
              </button>
              <app-clipboard-button
                :content="senderNote"
                :title="$t('tooltip-copy-note')"
              />
            </div>
          </div>
          <div class="card-body p-3">
            <!-- Raw Source Mode -->
            <pre v-if="showRawNote" class="mb-0 font-monospace text-wrap bg-dark text-light p-3 rounded border">{{ senderNote }}</pre>

            <!-- Plain Text Format -->
            <pre v-else-if="senderMessageFormat === 'text'" class="mb-0 font-monospace text-wrap bg-body-tertiary p-3 rounded border">{{ senderNote }}</pre>
            
            <!-- JSON Format -->
            <pre v-else-if="senderMessageFormat === 'json'" class="mb-0 font-monospace text-wrap bg-dark text-light p-3 rounded border">{{ formattedJSONMessage }}</pre>

            <!-- Markdown / HTML Format -->
            <div v-else class="formatted-note-content p-2" v-html="renderedFormattedMessage" />
          </div>
        </div>

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
        <div v-if="readsRemaining > 0" class="alert alert-info mt-3 shadow-sm" role="alert">
          <i class="fas fa-circle-info me-2" />
          {{ $t('text-reads-remaining-info', { count: readsRemaining }) }}
        </div>
        <!-- Burn Alert Warning Banner at Bottom -->
        <div v-if="readsRemaining === 0" class="alert alert-warning border-warning shadow-sm mt-3 mb-0 d-flex align-items-center" role="alert">
          <i class="fas fa-triangle-exclamation fa-lg text-warning me-3" />
          <div>
            <strong>Burned Secret Alert:</strong> This secret has been permanently deleted from server memory. Copy or save your content now before closing this tab.
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script lang="ts">
import DOMPurify from "dompurify";
import JSZip from "jszip";
import { defineComponent } from "vue";
import appCrypto from "../crypto.ts";
import OTSMeta from "../ots-meta";
import appClipboardButton from "./clipboard-button.vue";
import FilesDisplay from "./fileDisplay.vue";
import GrowArea from "./growarea.vue";
import appQrButton from "./qr-button.vue";

interface FileEntry {
	buffer?: ArrayBuffer;
	id: string;
	name: string;
	size: number;
	type: string;
	url: string;
}

export default defineComponent({
	components: { FilesDisplay, GrowArea, appClipboardButton, appQrButton },

	data() {
		return {
			files: [] as FileEntry[],
			inputPassword: "",
			isGeneratingBundle: false,
			popover: null,
			readsRemaining: 0,
			secret: null as null | string,
			secretContentBlobURL: null as null | string,
			secretLoading: false,
			senderMessageFormat: "text",
			senderNote: "",
			showRawNote: false,
		};
	},

	computed: {
		formattedJSONMessage(): string {
			if (!this.senderNote) return "";
			try {
				const parsed = JSON.parse(this.senderNote);
				return JSON.stringify(parsed, null, 2);
			} catch {
				return this.senderNote;
			}
		},

		renderedFormattedMessage(): string {
			if (!this.senderNote) return "";

			const purifyOptions = {
				ADD_ATTR: ["target"],
				ALLOWED_ATTR: ["href", "title", "target", "rel", "class"],
				ALLOWED_TAGS: [
					"div", "span", "p", "strong", "b", "em", "i", "u", "code", "pre",
					"ul", "ol", "li", "br", "hr", "a", "table", "thead", "tbody", "tr",
					"th", "td", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote"
				],
				ALLOW_DATA_ATTR: false,
				FORBID_ATTR: ["style", "onerror", "onload", "onclick", "onmouseover"],
				FORBID_TAGS: [
					"script", "style", "iframe", "object", "embed", "form", "input",
					"button", "select", "option", "meta", "link", "base", "svg", "math"
				],
			};

			if (this.senderMessageFormat === "md") {
				const convertedMD = this.senderNote
					.replace(/\*\*(.*?)\*\*/g, "<strong>$1</strong>")
					.replace(/\*(.*?)\*/g, "<em>$1</em>")
					.replace(/`(.*?)`/g, "<code class='bg-body-tertiary px-1 rounded'>$1</code>")
					.replace(/\n/g, "<br>");
				return DOMPurify.sanitize(convertedMD, purifyOptions);
			}

			if (this.senderMessageFormat === "html") {
				return DOMPurify.sanitize(this.senderNote, purifyOptions);
			}

			return DOMPurify.sanitize(this.senderNote.replace(/\n/g, "<br>"), purifyOptions);
		},
	},

	emits: ["error"],

	methods: {
		async computeSHA256(buffer: ArrayBuffer): Promise<string> {
			const hashBuffer = await window.crypto.subtle.digest("SHA-256", buffer);
			const bytes = new Uint8Array(hashBuffer);
			let hex = "";
			for (let i = 0; i < bytes.length; i++) {
				hex += bytes[i].toString(16).padStart(2, "0");
			}
			return hex;
		},

		async downloadBundle(): Promise<void> {
			if (this.isGeneratingBundle) return;
			this.isGeneratingBundle = true;

			try {
				const zip = new JSZip();
				const itemsToHash: { name: string; bytes: Uint8Array }[] = [];

				// 1. Prepare secret text
				if (this.secret) {
					itemsToHash.push({
						name: "secret.txt",
						bytes: new TextEncoder().encode(this.secret),
					});
				}

				// 2. Prepare sender context note (if present)
				if (this.senderNote) {
					let noteFilename = "note.txt";
					if (this.senderMessageFormat === "md") {
						noteFilename = "note.md";
					} else if (this.senderMessageFormat === "html") {
						noteFilename = "note.html";
					} else if (this.senderMessageFormat === "json") {
						noteFilename = "note.json";
					}
					itemsToHash.push({
						name: noteFilename,
						bytes: new TextEncoder().encode(this.senderNote),
					});
				}

				// 2. Prepare attached files
				for (const f of this.files) {
					let buf = f.buffer;
					if (!buf && f.url) {
						buf = await fetch(f.url).then((res) => res.arrayBuffer());
					}
					if (buf) {
						itemsToHash.push({
							name: f.name,
							bytes: new Uint8Array(buf),
						});
					}
				}

				// 3. Compute hardware SHA-256 hashes IN PARALLEL via Web Crypto API
				const hashResults = await Promise.all(
					itemsToHash.map(async (item) => {
						const hash = await this.computeSHA256(item.bytes.buffer);
						return `${hash}  ${item.name}`;
					}),
				);

				// 4. Add items to ZIP container with zero-copy STORE compression
				itemsToHash.forEach((item) => {
					zip.file(item.name, item.bytes, { compression: "STORE" });
				});

				// 5. Add SHA256SUMS manifest file
				zip.file("SHA256SUMS", hashResults.join("\n") + "\n");

				// 6. Asynchronously generate ZIP Blob
				const zipBlob = await zip.generateAsync({ type: "blob" });

				// Yield to event loop to allow Chrome disk stream to flush instantly
				await new Promise((resolve) => setTimeout(resolve, 0));

				const blobUrl = window.URL.createObjectURL(zipBlob);
				const link = document.createElement("a");
				const prefix = this.secretId ? this.secretId.substring(0, 8) : "bundle";
				link.href = blobUrl;
				link.download = `secret-bundle-${prefix}.zip`;
				document.body.appendChild(link);
				link.click();
				document.body.removeChild(link);

				// Revoke blob object URL after 3s to finalize browser download bar
				setTimeout(() => {
					window.URL.revokeObjectURL(blobUrl);
				}, 3000);
			} catch (err) {
				console.error("Failed to generate zip bundle:", err);
			} finally {
				this.isGeneratingBundle = false;
			}
		},

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
						this.readsRemaining = data.reads_remaining || 0;
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
								this.senderNote = meta.message;
								this.senderMessageFormat = meta.messageFormat || "text";

								meta.files.forEach((file) => {
									file.arrayBuffer().then((ab) => {
										const blobURL = window.URL.createObjectURL(
											new Blob([ab], { type: file.type }),
										);
										this.files.push({
											buffer: ab,
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
