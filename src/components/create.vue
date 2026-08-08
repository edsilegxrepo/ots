<template>
  <!-- Creation disabled -->
  <div
    v-if="!showCreateForm"
    class="card border-info-subtle mb-3"
  >
    <!-- Safe: Trusted internal translation string from i18n.yaml -->
    <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
    <div
      class="card-header bg-info-subtle"
      v-html="$t('title-secret-create-disabled')"
    />
    <!-- Safe: Trusted internal translation string from i18n.yaml -->
    <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
    <div
      class="card-body"
      v-html="$t('text-secret-create-disabled')"
    />
  </div>

  <!-- Creation possible -->
  <div
    v-else
    class="card border-primary-subtle mb-3"
  >
    <!-- Safe: Trusted internal translation string from i18n.yaml -->
    <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
    <div class="card-header bg-primary-subtle d-flex justify-content-between align-items-center">
      <span v-html="$t('title-new-secret')" />
      <button
        type="button"
        class="btn btn-sm shadow-sm"
        :class="senderNote ? 'btn-success' : 'btn-outline-primary'"
        @click="openMessageModal"
      >
        <i :class="senderNote ? 'fas fa-check-circle me-1' : 'fas fa-envelope-open-text me-1'" />
        {{ senderNote ? $t('btn-edit-sender-message') : $t('btn-sender-message') }}
      </button>
    </div>
    <div class="card-body">
      <!-- Safe: Administrator configured custom banner HTML in customize.yaml -->
      <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
      <div
        v-if="customize.customBannerHTML"
        class="alert alert-info mb-3"
        v-html="customize.customBannerHTML"
      />
      <form
        class="row"
        @submit.prevent="createSecret"
      >
        <div class="col-12 mb-3">
          <div class="d-flex justify-content-between align-items-center mb-1">
            <label for="createSecretData" class="form-label mb-0">{{ $t('label-secret-data') }}</label>
            <span class="small text-secondary"><kbd class="bg-body-secondary text-body border px-1">Ctrl</kbd> + <kbd class="bg-body-secondary text-body border px-1">Enter</kbd> to create</span>
          </div>
          <grow-area
            id="createSecretData"
            v-model="secret"
            class="form-control"
            :rows="3"
            @keydown.ctrl.enter="createSecret"
            @keydown.meta.enter="createSecret"
            @paste-file="handlePasteFile"
          />
        </div>

        <!-- Zero-Knowledge Security Assurance Badge -->
        <div class="col-12 mb-3">
          <div class="card bg-body-tertiary border-success-subtle shadow-sm">
            <div class="card-body p-2 px-3 d-flex flex-wrap align-items-center justify-content-between gap-2">
              <div class="d-flex align-items-center gap-2">
                <i class="fas fa-shield-halved text-success fa-lg me-1" />
                <span class="small fw-semibold text-secondary">
                  Zero-Knowledge Encryption: <strong>AES-256-GCM AEAD</strong> (32-Char CSPRNG Key • 300,000 PBKDF2 Iterations)
                </span>
              </div>
              <span class="badge bg-success-subtle text-success-emphasis border border-success-subtle font-monospace py-1 px-2" style="font-size: 0.75rem;">
                <i class="fas fa-lock me-1" /> Enterprise Grade (190.5 bits Entropy)
              </span>
            </div>
          </div>
        </div>
        <div
          v-if="!customize.disableFileAttachment"
          class="col-12 mb-3"
        >
          <label for="createSecretFiles">{{ $t('label-secret-files') }}</label>
          <input
            id="createSecretFiles"
            ref="createSecretFiles"
            class="form-control"
            type="file"
            multiple
            :accept="acceptedTypesPattern"
            @change="handleSelectFiles"
          >
          <div class="form-text mt-1">
            <span class="me-2">{{ $t('text-max-filesize', { maxSize: bytesToHuman(maxFileSize) }) }}</span>
            <template v-if="supportedExtensionsList.length > 0">
              <span class="text-muted me-2">•</span>
              <span class="fw-semibold text-secondary me-1">{{ $t('label-supported-extensions') }}</span>
              <span class="d-inline-flex flex-wrap gap-1 align-items-center">
                <span
                  v-for="ext in supportedExtensionsList"
                  :key="ext"
                  class="badge bg-secondary-subtle text-secondary-emphasis border border-secondary-subtle font-monospace py-1 px-2 me-1"
                  style="font-size: 0.75rem;"
                >
                  {{ ext }}
                </span>
              </span>
            </template>
          </div>
          <div
            v-if="invalidFilesSelected"
            class="alert alert-danger"
          >
            {{ $t('text-invalid-files-selected') }}
          </div>
          <div
            v-else-if="maxFileSizeExceeded"
            class="alert alert-danger"
          >
            {{ $t('text-max-filesize-exceeded', { curSize: bytesToHuman(fileSize), maxSize: bytesToHuman(maxFileSize) }) }}
          </div>
          <FilesDisplay
            v-if="attachedFiles.length > 0"
            class="mt-3"
            :can-delete="true"
            :track-download="false"
            :files="attachedFiles"
            @file-clicked="deleteFile"
          />
        </div>
        <div class="col-12">
          <div class="row align-items-center justify-content-between gy-2">
            <div class="col-12 col-md-auto">
              <button
                type="submit"
                class="btn btn-success"
                :disabled="!canCreate"
              >
                <template v-if="!createRunning">
                  {{ $t('btn-create-secret') }}
                </template>
                <template v-else>
                  <i class="fa-solid fa-spinner fa-spin-pulse" />
                  {{ $t('btn-create-secret-processing') }}
                </template>
              </button>
            </div>

            <div class="col-12 col-md-auto ms-md-auto d-flex flex-wrap align-items-center gap-3">
              <div
                v-if="customize.maxSecretReads > 0 && !customize.disableReusabilityOverride"
                class="d-flex align-items-center"
              >
                <label
                  class="col-form-label me-2 text-nowrap small fw-semibold"
                  for="createSecretReads"
                >{{ $t('label-reusability') }}</label>
                <div class="btn-group btn-group-sm" role="group" aria-label="Reusability choices">
                  <button
                    type="button"
                    class="btn shadow-sm"
                    :class="selectedReads === 1 ? 'btn-primary' : 'btn-outline-secondary'"
                    @click="selectedReads = 1"
                  >
                    1 Read
                  </button>
                  <button
                    v-for="r in Math.min(4, (customize.maxSecretReads - 1))"
                    :key="r + 1"
                    type="button"
                    class="btn shadow-sm"
                    :class="selectedReads === (r + 1) ? 'btn-primary' : 'btn-outline-secondary'"
                    @click="selectedReads = r + 1"
                  >
                    {{ r + 1 }} Reads
                  </button>
                </div>
              </div>

              <div
                v-if="!customize.disableExpiryOverride"
                class="d-flex align-items-center"
              >
                <label
                  class="col-form-label me-2 text-nowrap small fw-semibold"
                  for="createSecretExpiry"
                >{{ $t('label-expiry') }}</label>
                <select
                  id="createSecretExpiry"
                  v-model="selectedExpiry"
                  class="form-select form-select-sm shadow-sm"
                  style="min-width: 140px;"
                >
                  <option
                    v-for="opt in expiryChoices"
                    :key="opt.value || 'null'"
                    :value="opt.value"
                  >
                    {{ opt.text }}
                  </option>
                </select>
              </div>
            </div>
          </div>
        </div>
      </form>
    </div>
  </div>

  <!-- Sender Context Message Modal -->
  <div
    v-if="showMessageModal"
    class="modal fade show d-block"
    tabindex="-1"
    style="background-color: rgba(0, 0, 0, 0.5);"
    @click.self="closeMessageModal"
  >
    <div class="modal-dialog modal-dialog-centered modal-lg">
      <div class="modal-content shadow-lg">
        <div class="modal-header bg-primary-subtle">
          <h5 class="modal-title">
            <i class="fas fa-envelope-open-text me-2 text-primary" />
            {{ $t('title-sender-message-modal') }}
          </h5>
          <button
            type="button"
            class="btn-close"
            @click="closeMessageModal"
          />
        </div>
        <div class="modal-body">
          <!-- Format Selector Pills -->
          <div class="mb-3">
            <label class="form-label fw-bold small text-uppercase text-secondary me-3">
              {{ $t('label-message-format') }}
            </label>
            <div class="btn-group" role="group">
              <button
                type="button"
                class="btn btn-sm"
                :class="draftMessageFormat === 'text' ? 'btn-primary' : 'btn-outline-secondary'"
                @click="setFormatMode('text')"
              >
                <i class="fas fa-file-alt me-1" /> Plain Text
              </button>
              <button
                type="button"
                class="btn btn-sm"
                :class="draftMessageFormat === 'md' ? 'btn-primary' : 'btn-outline-secondary'"
                @click="setFormatMode('md')"
              >
                <i class="fab fa-markdown me-1" /> Markdown
              </button>
              <button
                type="button"
                class="btn btn-sm"
                :class="draftMessageFormat === 'html' ? 'btn-primary' : 'btn-outline-secondary'"
                @click="setFormatMode('html')"
              >
                <i class="fas fa-code me-1" /> HTML
              </button>
              <button
                type="button"
                class="btn btn-sm"
                :class="draftMessageFormat === 'json' ? 'btn-primary' : 'btn-outline-secondary'"
                @click="setFormatMode('json')"
              >
                <i class="fas fa-file-code me-1" /> JSON
              </button>
            </div>
          </div>

          <!-- Note Textarea -->
          <div class="mb-2">
            <div class="d-flex justify-content-between align-items-center mb-1">
              <div class="d-flex align-items-center gap-2">
                <label for="modalSenderNoteInput" class="form-label mb-0">
                  {{ $t('label-message-text') }}
                </label>
                <button
                  type="button"
                  class="btn btn-link btn-sm p-0 text-decoration-none small"
                  @click="insertFormatTemplate"
                >
                  <i class="fas fa-wand-magic-sparkles me-1" /> Sample Template
                </button>
              </div>
              <small
                class="fw-semibold"
                :class="{
                  'text-muted': draftSenderNote.length < 180,
                  'text-warning': draftSenderNote.length >= 180 && draftSenderNote.length < 200,
                  'text-danger': draftSenderNote.length >= 200
                }"
              >
                {{ draftSenderNote.length }} / 200
              </small>
            </div>
            <textarea
              id="modalSenderNoteInput"
              v-model="draftSenderNote"
              class="form-control font-monospace"
              rows="4"
              maxlength="200"
              placeholder="Add a context note (e.g. Ticket #402, Staging DB credentials)..."
            />
          </div>
        </div>
        <div class="modal-footer d-flex justify-content-between">
          <button
            type="button"
            class="btn btn-outline-danger btn-sm"
            @click="clearMessageModal"
          >
            <i class="fas fa-trash me-1" /> {{ $t('btn-clear-message') }}
          </button>
          <div>
            <button
              type="button"
              class="btn btn-secondary btn-sm me-2"
              @click="closeMessageModal"
            >
              Cancel
            </button>
            <button
              type="button"
              class="btn btn-primary btn-sm"
              @click="saveMessageModal"
            >
              <i class="fas fa-check me-1" /> {{ $t('btn-save-message') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import DOMPurify from "dompurify";
import { defineComponent } from "vue";
import appCrypto from "../crypto.ts";
import { bytesToHuman } from "../helpers";
import OTSMeta from "../ots-meta";
import FilesDisplay from "./fileDisplay.vue";
import GrowArea from "./growarea.vue";

const defaultFormatTemplates: Record<string, string> = {
	text: "Ticket #402 - Staging DB Credentials",
	md: "**Ticket**: #402\n**Environment**: Staging\n- *Note*: Valid for 24h",
	html: "<div><strong>Ticket:</strong> #402</div>\n<div><em>Environment:</em> Staging</div>",
	json: `{\n  "ticket": "#402",\n  "environment": "staging"\n}`,
};

const defaultExpiryChoices = [
	90 * 86400, // 90 days
	30 * 86400, // 30 days
	7 * 86400, // 7 days
	3 * 86400, // 3 days
	24 * 3600, // 1 day
	12 * 3600, // 12 hours
	4 * 3600, // 4 hours
	60 * 60, // 1 hour
	30 * 60, // 30 minutes
	5 * 60, // 5 minutes
];

/*
 * We define an internal max file-size which cannot get exceeded even
 * though the server might accept more: at around 70 MiB the base64
 * encoding broke and nothing works anymore. This might be fixed by
 * changing how the base64 implementation works (maybe use a WASM
 * object?) or switching to a browser-native implementation in case
 * that will appear somewhen in the future but for now we just "fix"
 * the issue by disallowing bigger files.
 */
const internalMaxFileSize = 64 * 1024 * 1024; // 64 MiB

const passwordCharset =
	"0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ";
const passwordLength = 32;
const passwordRandomLimit =
	Math.floor(256 / passwordCharset.length) * passwordCharset.length;

export default defineComponent({
	components: { FilesDisplay, GrowArea },

	computed: {
		canCreate(): boolean {
			return (
				(this.secret.trim().length > 0 || this.selectedFileMeta.length > 0) &&
				!this.maxFileSizeExceeded &&
				!this.invalidFilesSelected
			);
		},

		customize(): any {
			return window.OTSCustomize || {};
		},

		expiryChoices(): Record<string, string | null>[] {
			const choices: Record<string, string | null>[] = [];

			if (!this.customize.disableDefaultExpiry) {
				let defaultLabel = this.$t("expire-default");
				if (window.maxSecretExpire > 0) {
					const choice = window.maxSecretExpire;
					let defaultDuration = "";
					if (choice >= 86400) {
						defaultDuration = this.$t(
							"expire-n-days",
							Math.round(choice / 86400),
						);
					} else if (choice >= 3600) {
						defaultDuration = this.$t(
							"expire-n-hours",
							Math.round(choice / 3600),
						);
					} else if (choice >= 60) {
						defaultDuration = this.$t(
							"expire-n-minutes",
							Math.round(choice / 60),
						);
					} else {
						defaultDuration = this.$t("expire-n-seconds", choice);
					}
					defaultLabel += ` (${defaultDuration})`;
				}
				choices.push({ text: defaultLabel, value: null as string | null });
			}

			for (const choice of this.customize.expiryChoices ||
				defaultExpiryChoices) {
				if (window.maxSecretExpire > 0 && choice > window.maxSecretExpire) {
					continue;
				}

				const option = { text: "", value: String(choice) };
				if (choice >= 86400) {
					option.text = this.$t("expire-n-days", Math.round(choice / 86400));
				} else if (choice >= 3600) {
					option.text = this.$t("expire-n-hours", Math.round(choice / 3600));
				} else if (choice >= 60) {
					option.text = this.$t("expire-n-minutes", Math.round(choice / 60));
				} else {
					option.text = this.$t("expire-n-seconds", choice);
				}

				choices.push(option);
			}

			return choices;
		},

		invalidFilesSelected(): boolean {
			if (
				!this.customize ||
				!this.customize.acceptedFileTypes ||
				this.customize.acceptedFileTypes === ""
			) {
				// No limitation configured, no need to check
				return false;
			}

			const accepted = this.customize.acceptedFileTypes.split(",");
			for (const fm of this.selectedFileMeta) {
				let isAccepted = false;

				for (const a of accepted) {
					isAccepted ||= this.isAcceptedBy(fm, a);
				}

				if (!isAccepted) {
					// Well we only needed one rejected
					return true;
				}
			}

			// We found no reason to reject: This is fine!
			return false;
		},

		isSecureEnvironment(): boolean {
			return Boolean(window.crypto.subtle);
		},

		maxFileSize(): number {
			const size = Number(this.customize?.maxAttachmentSizeTotal);
			return !size || size === 0
				? internalMaxFileSize
				: Math.min(internalMaxFileSize, size);
		},

		maxFileSizeExceeded(): boolean {
			return this.fileSize > this.maxFileSize;
		},

		showCreateForm(): boolean {
			return this.canWrite && this.isSecureEnvironment;
		},

		acceptedTypesPattern(): string {
			if (
				Array.isArray((this.customize as any).resolvedAcceptedExtensions) &&
				(this.customize as any).resolvedAcceptedExtensions.length > 0
			) {
				return (this.customize as any).resolvedAcceptedExtensions.join(",");
			}
			return this.customize.acceptedFileTypes || "";
		},

		/**
		 * supportedExtensionsList resolves the array of allowed extension strings (.png, .pdf, .zip)
		 * to display as badge pills in the file attachment section. It prioritizes server-pre-expanded
		 * resolvedAcceptedExtensions and falls back to splitting raw acceptedFileTypes string.
		 */
		supportedExtensionsList(): string[] {
			if (
				Array.isArray((this.customize as any).resolvedAcceptedExtensions) &&
				(this.customize as any).resolvedAcceptedExtensions.length > 0
			) {
				return (this.customize as any).resolvedAcceptedExtensions;
			}
			if (this.customize && this.customize.acceptedFileTypes) {
				return this.customize.acceptedFileTypes
					.split(",")
					.map((s: string) => s.trim())
					.filter(Boolean);
			}
			return [];
		},

		passphraseEntropyBits(): number {
			if (!this.customPassphrase) return 0;
			let poolSize = 0;
			if (/[a-z]/.test(this.customPassphrase)) poolSize += 26;
			if (/[A-Z]/.test(this.customPassphrase)) poolSize += 26;
			if (/[0-9]/.test(this.customPassphrase)) poolSize += 10;
			if (/[^a-zA-Z0-9]/.test(this.customPassphrase)) poolSize += 32;
			if (poolSize === 0) poolSize = passwordCharset.length;
			return Math.round(this.customPassphrase.length * Math.log2(poolSize));
		},

		passphraseStrengthPercent(): number {
			const bits = this.passphraseEntropyBits;
			if (bits === 0) return 0;
			return Math.min(100, Math.round((bits / 190.5) * 100));
		},

		passphraseStrengthLabel(): string {
			const bits = this.passphraseEntropyBits;
			if (bits < 50) return "Weak";
			if (bits < 90) return "Moderate";
			if (bits < 140) return "Good";
			return "Strong (Enterprise)";
		},

		passphraseStrengthClass(): string {
			const bits = this.passphraseEntropyBits;
			if (bits < 50) return "bg-danger";
			if (bits < 90) return "bg-warning";
			if (bits < 140) return "bg-info";
			return "bg-success";
		},

		passphraseBadgeClass(): string {
			const bits = this.passphraseEntropyBits;
			if (bits < 50) return "bg-danger-subtle text-danger border border-danger-subtle";
			if (bits < 90) return "bg-warning-subtle text-warning-emphasis border border-warning-subtle";
			if (bits < 140) return "bg-info-subtle text-info-emphasis border border-info-subtle";
			return "bg-success-subtle text-success-emphasis border border-success-subtle";
		},
	},

	created(): void {
		this.checkWriteAccess();
	},

	data() {
		return {
			attachedFiles: [],
			canWrite: null,
			createRunning: false,
			customPassphrase: "",
			draftMessageFormat: "text",
			draftSenderNote: "",
			fileSize: 0,
			secret: "",
			securePassword: null,
			selectedExpiry: null,
			selectedFileMeta: [],
			selectedReads: 1,
			senderMessageFormat: "text",
			senderNote: "",
			showMessageModal: false,
		};
	},

	emits: ["error", "navigate"],

	methods: {
		bytesToHuman,

		generateCustomPassphrase(): void {
			let pass = "";
			while (pass.length < passwordLength) {
				const values = window.crypto.getRandomValues(new Uint8Array(passwordLength));
				for (const n of values) {
					if (n >= passwordRandomLimit) continue;
					pass += passwordCharset[n % passwordCharset.length];
					if (pass.length === passwordLength) break;
				}
			}
			this.customPassphrase = pass;
		},

		openMessageModal(): void {
			this.draftSenderNote = this.senderNote;
			this.draftMessageFormat = this.senderMessageFormat || "text";
			if (!this.draftSenderNote && defaultFormatTemplates[this.draftMessageFormat]) {
				this.draftSenderNote = defaultFormatTemplates[this.draftMessageFormat];
			}
			this.showMessageModal = true;
		},

		setFormatMode(fmt: string): void {
			const isTemplateOrEmpty =
				!this.draftSenderNote ||
				Object.values(defaultFormatTemplates).includes(this.draftSenderNote.trim());

			this.draftMessageFormat = fmt;

			if (isTemplateOrEmpty && defaultFormatTemplates[fmt]) {
				this.draftSenderNote = defaultFormatTemplates[fmt];
			}
		},

		insertFormatTemplate(): void {
			if (defaultFormatTemplates[this.draftMessageFormat]) {
				this.draftSenderNote = defaultFormatTemplates[this.draftMessageFormat];
			}
		},

		closeMessageModal(): void {
			this.showMessageModal = false;
		},

		clearMessageModal(): void {
			this.draftSenderNote = "";
		},

		saveMessageModal(): void {
			const rawNote = this.draftSenderNote.trim().slice(0, 200);

			// Discard note if empty or matches an unmodified default template
			const isUnmodifiedTemplate = Object.values(defaultFormatTemplates).includes(rawNote);
			if (!rawNote || isUnmodifiedTemplate) {
				this.senderNote = "";
				this.showMessageModal = false;
				return;
			}

			if (this.draftMessageFormat === "html" || this.draftMessageFormat === "md") {
				this.senderNote = DOMPurify.sanitize(rawNote, {
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
				});
			} else {
				this.senderNote = rawNote;
			}

			this.senderMessageFormat = this.draftMessageFormat;
			this.showMessageModal = false;
		},

		checkWriteAccess(): Promise<void> {
			return fetch("/api/isWritable", {
				credentials: "same-origin",
				method: "GET",
				redirect: "error",
			})
				.then((resp) => {
					if (resp.status !== 204) {
						throw new Error(`unexpected status: ${resp.status}`);
					}
					this.canWrite = true;
				})
				.catch(() => {
					this.canWrite = false;
				});
		},

		// createSecret executes the secret creation after encrypting the secret
		createSecret(): void {
			if (!this.canCreate) {
				return;
			}

			// Encoding large files takes a while, prevent duplicate click on "create"
			this.createRunning = true;

			let password = this.customPassphrase ? this.customPassphrase.trim() : "";

			if (!password) {
				while (password.length < passwordLength) {
					const values = window.crypto.getRandomValues(
						new Uint8Array(passwordLength),
					);

					for (const n of values) {
						if (n >= passwordRandomLimit) {
							continue;
						}

						password += passwordCharset[n % passwordCharset.length];

						if (password.length === passwordLength) {
							break;
						}
					}
				}
			}

			this.securePassword = password;

			const meta = new OTSMeta();
			meta.secret = this.secret;
			if (this.senderNote) {
				meta.message = this.senderNote;
				meta.messageFormat = this.senderMessageFormat;
			}

			if (this.attachedFiles.length > 0) {
				for (const f of this.attachedFiles) {
					meta.files.push(f.fileObj);
				}
			}

			meta
				.serialize()
				.then((secret) => appCrypto.enc(secret, this.securePassword))
				.then((secret) => {
					let reqURL = "/api/create";
					if (this.selectedExpiry !== null) {
						reqURL = `/api/create?expire=${this.selectedExpiry}`;
					}

					const bodyPayload: any = { secret };
					if (this.selectedReads > 1) {
						bodyPayload.reads = this.selectedReads;
					}

					return fetch(reqURL, {
						body: JSON.stringify(bodyPayload),
						headers: {
							"content-type": "application/json",
						},
						method: "POST",
					})
						.then((resp) => {
							if (resp.status !== 201) {
								// Server says "no"
								this.$emit("error", this.$t("alert-something-went-wrong"));
								return;
							}

							resp.json().then((data) => {
								this.$emit("navigate", {
									path: "/display-secret-url",
									query: {
										expiresAt: data.expires_at,
										secretId: data.secret_id,
										securePassword: this.securePassword,
									},
								});
							});
						})
						.catch(() => {
							// Network error
							this.$emit("error", this.$t("alert-something-went-wrong"));
						});
				});
		},

		deleteFile(fileId: string): void {
			this.attachedFiles = [...this.attachedFiles].filter(
				(file) => file.id !== fileId,
			);
			this.updateFileMeta();
		},

		handlePasteFile(file: File): void {
			this.attachedFiles.push({
				fileObj: file,
				id: window.crypto.randomUUID(),
				name: file.name,
				size: file.size,
				type: file.type,
			});
			this.updateFileMeta();
		},

		handleSelectFiles(): void {
			for (const file of this.$refs.createSecretFiles.files) {
				this.attachedFiles.push({
					fileObj: file,
					id: window.crypto.randomUUID(),
					name: file.name,
					size: file.size,
					type: file.type,
				});
			}
			this.updateFileMeta();

			this.$refs.createSecretFiles.value = "";
		},

		isAcceptedBy(fileMeta: any, accept: string): boolean {
			if (
				Array.isArray((this.customize as any).resolvedAcceptedExtensions) &&
				(this.customize as any).resolvedAcceptedExtensions.length > 0
			) {
				const fileNameLower = (fileMeta.name || "").toLowerCase();
				return (this.customize as any).resolvedAcceptedExtensions.some(
					(ext: string) => fileNameLower.endsWith(ext.toLowerCase()),
				);
			}

			const raw = accept.trim().toLowerCase();
			if (!raw) return true;

			let ext = raw.replace(/^\*/, "").trim();
			if (!ext.startsWith(".")) ext = "." + ext;
			return (fileMeta.name || "").toLowerCase().endsWith(ext);
		},

		updateFileMeta(): void {
			let cumSize = 0;
			for (const f of this.attachedFiles) {
				cumSize += f.size;
			}

			this.fileSize = cumSize;
			this.selectedFileMeta = this.attachedFiles.map((file) => ({
				name: file.name,
				type: file.type,
			}));
		},
	},

	name: "AppCreate",
});
</script>
