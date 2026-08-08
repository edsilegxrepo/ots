<template>
  <div>
    <!-- Trigger Button -->
    <button
      class="btn btn-success btn-sm text-white shadow-sm fw-semibold"
      type="button"
      :title="$t('tooltip-generate-message')"
      data-bs-toggle="modal"
      data-bs-target="#enterpriseMessageModal"
    >
      <i class="fas fa-envelope-open-text fa-fw me-1" />
      <span class="d-none d-md-inline">{{ $t('btn-generate-message') }}</span>
    </button>

    <!-- Enterprise Message Modal -->
    <div
      id="enterpriseMessageModal"
      class="modal fade"
      tabindex="-1"
      aria-labelledby="enterpriseMessageModalLabel"
      aria-hidden="true"
    >
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header bg-info-subtle">
            <h5
              id="enterpriseMessageModalLabel"
              class="modal-title"
            >
              <i class="fas fa-file-signature me-2" />{{ $t('title-message-modal') }}
            </h5>
            <button
              type="button"
              class="btn-close"
              data-bs-dismiss="modal"
              aria-label="Close"
            />
          </div>
          <div class="modal-body">
            <!-- Format Selector Controls -->
            <div class="d-flex flex-wrap justify-content-between align-items-center mb-3 gap-2">
              <!-- Nav Tabs -->
              <ul
                class="nav nav-pills"
                role="tablist"
              >
                <li
                  class="nav-item"
                  role="presentation"
                >
                  <button
                    class="nav-link active"
                    data-bs-toggle="tab"
                    data-bs-target="#tab-full-link"
                    type="button"
                    role="tab"
                  >
                    <i class="fas fa-link me-1" /> Full Link
                  </button>
                </li>
                <li
                  class="nav-item"
                  role="presentation"
                >
                  <button
                    class="nav-link"
                    data-bs-toggle="tab"
                    data-bs-target="#tab-dual-link"
                    type="button"
                    role="tab"
                  >
                    <i class="fas fa-shield-halved me-1" /> Dual (Link)
                  </button>
                </li>
                <li
                  class="nav-item"
                  role="presentation"
                >
                  <button
                    class="nav-link"
                    data-bs-toggle="tab"
                    data-bs-target="#tab-dual-key"
                    type="button"
                    role="tab"
                  >
                    <i class="fas fa-key me-1" /> Dual (Key)
                  </button>
                </li>
                <li
                  class="nav-item"
                  role="presentation"
                >
                  <button
                    class="nav-link"
                    data-bs-toggle="tab"
                    data-bs-target="#tab-dual-combined"
                    type="button"
                    role="tab"
                  >
                    <i class="fas fa-comments me-1" /> Combined Chat
                  </button>
                </li>
              </ul>

              <!-- Format Switcher Radio Group -->
              <div
                class="btn-group btn-group-sm"
                role="group"
                aria-label="Format Switcher"
              >
                <input
                  id="fmtText"
                  v-model="selectedFormat"
                  type="radio"
                  class="btn-check"
                  name="formatOpts"
                  value="text"
                  autocomplete="off"
                >
                <label
                  class="btn btn-outline-primary"
                  for="fmtText"
                ><i class="fas fa-file-lines me-1" /> Text</label>

                <input
                  id="fmtHTML"
                  v-model="selectedFormat"
                  type="radio"
                  class="btn-check"
                  name="formatOpts"
                  value="html"
                  autocomplete="off"
                >
                <label
                  class="btn btn-outline-primary"
                  for="fmtHTML"
                ><i class="fas fa-code me-1" /> HTML</label>

                <input
                  id="fmtMD"
                  v-model="selectedFormat"
                  type="radio"
                  class="btn-check"
                  name="formatOpts"
                  value="md"
                  autocomplete="off"
                >
                <label
                  class="btn btn-outline-primary"
                  for="fmtMD"
                ><i class="fab fa-markdown me-1" /> Markdown</label>

                <input
                  id="fmtJSON"
                  v-model="selectedFormat"
                  type="radio"
                  class="btn-check"
                  name="formatOpts"
                  value="json"
                  autocomplete="off"
                >
                <label
                  class="btn btn-outline-primary"
                  for="fmtJSON"
                ><i class="fas fa-file-code me-1" /> JSON</label>
              </div>
            </div>

            <!-- Tab Content -->
            <div class="tab-content">
              <!-- Tab 1: Full Link Message -->
              <div
                id="tab-full-link"
                class="tab-pane fade show active"
                role="tabpanel"
              >
                <p class="small text-secondary mb-2">
                  Complete delivery template containing full secret decryption URL. (Format: {{ formatLabel }}).
                </p>
                <div class="position-relative mb-2">
                  <textarea
                    class="form-control font-monospace small"
                    rows="13"
                    readonly
                    :value="fullLinkTemplate"
                  />
                </div>
                <div class="d-flex justify-content-end gap-2">
                  <button
                    v-if="selectedFormat === 'html' || selectedFormat === 'md'"
                    type="button"
                    class="btn btn-outline-success shadow-sm"
                    :class="{'btn-success text-white': copyRichSuccess}"
                    title="Copy formatted rich text to paste into Outlook, Teams, Word, or Slack"
                    @click="copyRichHTML(fullLinkTemplate)"
                  >
                    <i :class="copyRichSuccess ? 'fas fa-check me-1' : 'fas fa-wand-magic-sparkles me-1'" />
                    {{ copyRichSuccess ? 'Copied for Outlook/Teams!' : 'Copy Rich HTML (Outlook/Teams)' }}
                  </button>
                  <app-clipboard-button
                    :content="fullLinkTemplate"
                    title="Copy Full Link Message"
                    :show-label="true"
                    label-text="Copy Raw"
                  />
                </div>
              </div>

              <!-- Tab 2: Dual Channel Link Message -->
              <div
                id="tab-dual-link"
                class="tab-pane fade"
                role="tabpanel"
              >
                <p class="small text-secondary mb-2">
                  Channel 1 Template (Email/Ticket): Short link without key. (Format: {{ formatLabel }}).
                </p>
                <div class="position-relative mb-2">
                  <textarea
                    class="form-control font-monospace small"
                    rows="13"
                    readonly
                    :value="dualLinkTemplate"
                  />
                </div>
                <div class="d-flex justify-content-end gap-2">
                  <button
                    v-if="selectedFormat === 'html' || selectedFormat === 'md'"
                    type="button"
                    class="btn btn-outline-success shadow-sm"
                    :class="{'btn-success text-white': copyRichSuccess}"
                    title="Copy formatted rich text to paste into Outlook, Teams, Word, or Slack"
                    @click="copyRichHTML(dualLinkTemplate)"
                  >
                    <i :class="copyRichSuccess ? 'fas fa-check me-1' : 'fas fa-wand-magic-sparkles me-1'" />
                    {{ copyRichSuccess ? 'Copied for Outlook/Teams!' : 'Copy Rich HTML (Outlook/Teams)' }}
                  </button>
                  <app-clipboard-button
                    :content="dualLinkTemplate"
                    title="Copy Dual Link Message"
                    :show-label="true"
                    label-text="Copy Raw"
                  />
                </div>
              </div>

              <!-- Tab 3: Dual Channel Key Message -->
              <div
                id="tab-dual-key"
                class="tab-pane fade"
                role="tabpanel"
              >
                <p class="small text-secondary mb-2">
                  Channel 2 Template (SMS/Teams/Slack): Decryption key only. (Format: {{ formatLabel }}).
                </p>
                <div class="position-relative mb-2">
                  <textarea
                    class="form-control font-monospace small"
                    rows="10"
                    readonly
                    :value="dualKeyTemplate"
                  />
                </div>
                <div class="d-flex justify-content-end gap-2">
                  <button
                    v-if="selectedFormat === 'html' || selectedFormat === 'md'"
                    type="button"
                    class="btn btn-outline-success shadow-sm"
                    :class="{'btn-success text-white': copyRichSuccess}"
                    title="Copy formatted rich text to paste into Outlook, Teams, Word, or Slack"
                    @click="copyRichHTML(dualKeyTemplate)"
                  >
                    <i :class="copyRichSuccess ? 'fas fa-check me-1' : 'fas fa-wand-magic-sparkles me-1'" />
                    {{ copyRichSuccess ? 'Copied for Outlook/Teams!' : 'Copy Rich HTML (Outlook/Teams)' }}
                  </button>
                  <app-clipboard-button
                    :content="dualKeyTemplate"
                    title="Copy Decryption Key Message"
                    :show-label="true"
                    label-text="Copy Raw"
                  />
                </div>
              </div>

              <!-- Tab 4: Combined Chat Notice -->
              <div
                id="tab-dual-combined"
                class="tab-pane fade"
                role="tabpanel"
              >
                <p class="small text-secondary mb-2">
                  Combined notice formatted for instant pasting into internal chat tools or API pipelines. (Format: {{ formatLabel }}).
                </p>
                <div class="position-relative mb-2">
                  <textarea
                    class="form-control font-monospace small"
                    rows="11"
                    readonly
                    :value="combinedChatTemplate"
                  />
                </div>
                <div class="d-flex justify-content-end gap-2">
                  <button
                    v-if="selectedFormat === 'html' || selectedFormat === 'md'"
                    type="button"
                    class="btn btn-outline-success shadow-sm"
                    :class="{'btn-success text-white': copyRichSuccess}"
                    title="Copy formatted rich text to paste into Outlook, Teams, Word, or Slack"
                    @click="copyRichHTML(combinedChatTemplate)"
                  >
                    <i :class="copyRichSuccess ? 'fas fa-check me-1' : 'fas fa-wand-magic-sparkles me-1'" />
                    {{ copyRichSuccess ? 'Copied for Outlook/Teams!' : 'Copy Rich HTML (Outlook/Teams)' }}
                  </button>
                  <app-clipboard-button
                    :content="combinedChatTemplate"
                    title="Copy Combined Chat Notice"
                    :show-label="true"
                    label-text="Copy Raw"
                  />
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button
              type="button"
              class="btn btn-secondary"
              data-bs-dismiss="modal"
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import AppClipboardButton from "./clipboard-button.vue";

export default defineComponent({
	components: { AppClipboardButton },

	computed: {
		isoExpiration(): string {
			if (this.expiresAt) {
				return this.expiresAt.toISOString();
			}
			if (this.burnTime) {
				return this.burnTime;
			}
			return "Configured retention policy";
		},

		secretId(): string {
			const match = this.secretUrl.match(/#([a-f0-9-]+)/i);
			return match ? match[1].substring(0, 8) : "ID";
		},

		combinedChatTemplate(): string {
			const expNote = this.burnTime
				? `Secret expires on ${this.burnTime}.`
				: "Secret expires according to configured retention policy.";

			if (this.selectedFormat === "json") {
				return JSON.stringify(
					{
						burn_on_read: true,
						decryption_key: this.securePassword,
						expiration: this.isoExpiration,
						header: `DUAL-CHANNEL SECURE DELIVERY NOTICE [${this.secretId}]`,
						secret_id: this.secretId,
						short_url: this.shortUrl,
						type: "dual_channel_combined_notice",
					},
					null,
					2,
				);
			}

			if (this.selectedFormat === "html") {
				return `<div style="font-family: Arial, sans-serif; border: 1px solid #0dcaf0; border-radius: 6px; padding: 15px; background-color: #f8f9fa;">
  <h4 style="color: #055160; margin-top: 0;">DUAL-CHANNEL SECURE DELIVERY NOTICE [${this.secretId}]</h4>
  <div style="background-color: #e7f1ff; border-left: 4px solid #0dcaf0; padding: 8px; margin-bottom: 10px; font-size: 13px;">
    <strong>NOTE:</strong> ${expNote}
  </div>
  <p style="margin-bottom: 5px;"><strong>Link (Channel 1):</strong> <a href="${this.shortUrl}">${this.shortUrl}</a></p>
  <p style="margin-bottom: 10px;"><strong>Key (Channel 2):</strong> <code>${this.securePassword}</code></p>
  <p style="font-size: 12px; color: #6c757d; margin: 0;">NOTICE: Opening the link decrypts and burns the secret immediately.</p>
</div>`;
			}

			if (this.selectedFormat === "md") {
				return `### DUAL-CHANNEL SECURE DELIVERY NOTICE [${this.secretId}]\n\n> [!WARNING]\n> ${expNote}\n\n- **Link (Channel 1):** ${this.shortUrl}\n- **Key  (Channel 2):** \`${this.securePassword}\` \n\n*NOTICE: Opening the link decrypts and burns the secret immediately.*`;
			}

			// Plain Text Default
			return `\`\`\`\n===================================================================\n       DUAL-CHANNEL SECURE DELIVERY NOTICE [${this.secretId}]\n===================================================================\n\nNOTE: ${expNote}\n\n  Link (Channel 1): ${this.shortUrl}\n  Key  (Channel 2): ${this.securePassword}\n\nNOTICE: Opening the link decrypts and burns the secret immediately.\n===================================================================\n\`\`\``;
		},

		dualKeyTemplate(): string {
			const expNote = this.burnTime
				? `Secret expires on ${this.burnTime}.`
				: "Secret expires according to configured retention policy.";

			if (this.selectedFormat === "json") {
				return JSON.stringify(
					{
						decryption_key: this.securePassword,
						expiration: this.isoExpiration,
						header: `DECRYPTION KEY TRANSMISSION [${this.secretId}]`,
						instructions:
							"Paste this key when prompted after opening your secret link.",
						secret_id: this.secretId,
						type: "dual_channel_decryption_key",
					},
					null,
					2,
				);
			}

			if (this.selectedFormat === "html") {
				return `<div style="font-family: Arial, sans-serif; border: 1px solid #ffc107; border-radius: 6px; padding: 15px; background-color: #fff3cd;">
  <h3 style="color: #664d03; margin-top: 0;">DECRYPTION KEY TRANSMISSION (PART 2 OF 2) [${this.secretId}]</h3>
  <div style="background-color: #fff8e1; border-left: 4px solid #ffc107; padding: 8px; margin: 10px 0; font-size: 13px;">
    <strong>NOTE:</strong> ${expNote}
  </div>
  <p>Here is your decryption key for the secret link sent via email:</p>
  <div style="background: #ffffff; border: 1px solid #ffe69c; padding: 10px; font-family: monospace; font-size: 14px; margin: 10px 0;">
    <strong>DECRYPTION KEY:</strong><br>${this.securePassword}
  </div>
  <p style="font-size: 12px; color: #664d03;">INSTRUCTIONS: Paste this key when prompted after opening your secret link.</p>
</div>`;
			}

			if (this.selectedFormat === "md") {
				return `### DECRYPTION KEY TRANSMISSION (PART 2 OF 2) [${this.secretId}]\n\n> [!WARNING]\n> ${expNote}\n\nHere is your decryption key for the secret link sent via email:\n\n> **DECRYPTION KEY:**\n> \`${this.securePassword}\` \n\n**INSTRUCTIONS:** Paste this key when prompted after opening your secret link.`;
			}

			// Plain Text Default
			return `===================================================================\n      DECRYPTION KEY TRANSMISSION (PART 2 OF 2) [${this.secretId}]\n===================================================================\n\nNOTE: ${expNote}\n\nHere is your decryption key for the secret link sent via email:\n\n-------------------------------------------------------------------\nDECRYPTION KEY:\n${this.securePassword}\n-------------------------------------------------------------------\n\nINSTRUCTIONS:\nPaste this key when prompted after opening your secret link.\n\n===================================================================`;
		},

		dualLinkTemplate(): string {
			const expNote = this.burnTime
				? `Secret expires on ${this.burnTime}.`
				: "Secret expires according to configured retention policy.";

			if (this.selectedFormat === "json") {
				const obj: Record<string, unknown> = {
					burn_on_read: true,
					decryption_key_channel: "separate_channel_required",
					expiration: this.isoExpiration,
					header: `DUAL-CHANNEL SECRET TRANSMISSION [${this.secretId}]`,
					secret_id: this.secretId,
					short_url: this.shortUrl,
					type: "dual_channel_secret_link",
				};
				return JSON.stringify(obj, null, 2);
			}

			if (this.selectedFormat === "html") {
				return `<div style="font-family: Arial, sans-serif; border: 1px solid #198754; border-radius: 6px; padding: 15px; background-color: #ffffff;">
  <h3 style="color: #0f5132; margin-top: 0;">DUAL-CHANNEL SECRET TRANSMISSION (PART 1 OF 2) [${this.secretId}]</h3>
  <div style="background-color: #e8f5e9; border-left: 4px solid #198754; padding: 8px; margin: 10px 0; font-size: 13px;">
    <strong>NOTE:</strong> ${expNote}
  </div>
  <p>A secure, encrypted one-time secret has been created for you. For enhanced security, the decryption key is delivered via a separate channel (SMS, Slack, or Teams).</p>
  <div style="background-color: #f8f9fa; border-left: 4px solid #198754; padding: 10px; margin: 10px 0; word-break: break-all;">
    <strong>SECRET LINK (Without Decryption Key):</strong><br>
    <a href="${this.shortUrl}" style="color: #0d6efd;">${this.shortUrl}</a>
  </div>
  <p style="font-size: 13px; color: #6c757d;">
    <strong>INSTRUCTIONS:</strong><br>
    1. Obtain the Decryption Key from your second communication channel.<br>
    2. Accessing the link burns (deletes) the secret permanently.
  </p>
</div>`;
			}

			if (this.selectedFormat === "md") {
				return `### DUAL-CHANNEL SECRET TRANSMISSION (PART 1 OF 2) [${this.secretId}]\n\n> [!WARNING]\n> ${expNote}\n\nA secure, encrypted one-time secret has been created for you.\nFor enhanced security, the decryption key is delivered via a separate channel (SMS, Slack, or Teams).\n\n> **SECRET LINK (Without Decryption Key):**\n> ${this.shortUrl}\n\n**INSTRUCTIONS:**\n1. Obtain the Decryption Key from your second communication channel.\n2. Accessing the link burns (deletes) the secret permanently.`;
			}

			// Plain Text Default
			return `===================================================================\n      DUAL-CHANNEL SECRET TRANSMISSION (PART 1 OF 2) [${this.secretId}]\n===================================================================\n\nNOTE: ${expNote}\n\nA secure, encrypted one-time secret has been created for you.\nFor enhanced security, the decryption key is delivered via a \nseparate channel (SMS, Slack, or Teams).\n\n-------------------------------------------------------------------\nSECRET LINK (Without Decryption Key):\n${this.shortUrl}\n-------------------------------------------------------------------\n\nINSTRUCTIONS:\n1. Obtain the Decryption Key from your second communication channel.\n2. Accessing the link burns (deletes) the secret permanently.\n\n===================================================================`;
		},

		formatLabel(): string {
			switch (this.selectedFormat) {
				case "json":
					return "JSON Payload";
				case "html":
					return "HTML Email Box";
				case "md":
					return "Markdown";
				default:
					return "Plain Text / ASCII";
			}
		},

		fullLinkTemplate(): string {
			const expNote = this.burnTime
				? `Secret expires on ${this.burnTime}.`
				: "Secret expires according to configured retention policy.";

			if (this.selectedFormat === "json") {
				const obj: Record<string, unknown> = {
					burn_on_read: true,
					expiration: this.isoExpiration,
					header: `CONFIDENTIAL ONE-TIME SECRET [${this.secretId}]`,
					instructions:
						"Accessing this URL decrypts the payload and PERMANENTLY BURNS (deletes) the secret from the server.",
					secret_id: this.secretId,
					secret_url: this.secretUrl,
					type: "one_time_secret",
				};
				return JSON.stringify(obj, null, 2);
			}

			if (this.selectedFormat === "html") {
				return `<div style="font-family: Arial, sans-serif; border: 1px solid #198754; border-radius: 6px; padding: 15px; background-color: #ffffff;">
  <h3 style="color: #0f5132; margin-top: 0;">CONFIDENTIAL ONE-TIME SECRET [${this.secretId}]</h3>
  <div style="background-color: #e8f5e9; border-left: 4px solid #198754; padding: 8px; margin: 10px 0; font-size: 13px;">
    <strong>NOTE:</strong> ${expNote}
  </div>
  <p>A secure, encrypted one-time secret has been generated for you.</p>
  <div style="background-color: #f8f9fa; border-left: 4px solid #198754; padding: 10px; margin: 10px 0; word-break: break-all;">
    <strong>SECRET URL:</strong><br>
    <a href="${this.secretUrl}" style="color: #0d6efd;">${this.secretUrl}</a>
  </div>
  <p style="font-size: 13px; color: #6c757d;">
    <strong>IMPORTANT INSTRUCTIONS:</strong><br>
    1. Accessing this URL decrypts the payload and <strong>PERMANENTLY BURNS (deletes)</strong> the secret.<br>
    2. Please copy or store the content immediately upon opening.
  </p>
</div>`;
			}

			if (this.selectedFormat === "md") {
				return `### CONFIDENTIAL ONE-TIME SECRET [${this.secretId}]\n\n> [!WARNING]\n> ${expNote}\n\nA secure, encrypted one-time secret has been generated for you.\n\n> **SECRET URL:**\n> ${this.secretUrl}\n\n**IMPORTANT INSTRUCTIONS:**\n1. Accessing this URL decrypts the payload and **PERMANENTLY BURNS (deletes)** the secret from the server.\n2. Please copy or store the content immediately upon opening.`;
			}

			// Plain Text Default
			return `===================================================================\n             CONFIDENTIAL ONE-TIME SECRET [${this.secretId}]\n===================================================================\n\nNOTE: ${expNote}\n\nA secure, encrypted one-time secret has been generated for you.\n\n-------------------------------------------------------------------\nSECRET URL:\n${this.secretUrl}\n-------------------------------------------------------------------\n\nIMPORTANT INSTRUCTIONS:\n1. Accessing this URL decrypts the payload and PERMANENTLY BURNS\n   (deletes) the secret from the server.\n2. Please copy or store the content immediately upon opening.\n\n===================================================================`;
		},

		copyRichHTML(templateString: string): void {
			let htmlString = templateString;
			if (this.selectedFormat === "md") {
				htmlString = `<div style="font-family: Arial, sans-serif; border: 1px solid #0d6efd; border-radius: 6px; padding: 15px; background-color: #f8f9fa;">` +
					templateString
						.replace(/### (.*?)\n/g, "<h3 style='color: #0b5ed7; margin-top: 0;'>$1</h3>")
						.replace(/> \[!WARNING\]\n> (.*?)\n/g, "<div style='background-color: #fff3cd; border-left: 4px solid #ffc107; padding: 8px; margin: 10px 0;'><strong>NOTE:</strong> $1</div>")
						.replace(/\*\*(.*?)\*\*/g, "<strong>$1</strong>")
						.replace(/`(.*?)`/g, "<code style='background: #e9ecef; padding: 2px 4px; border-radius: 4px;'>$1</code>")
						.replace(/- \*\*(.*?)\*\*: (.*?)\n/g, "<p style='margin: 4px 0;'><strong>$1:</strong> <a href='$2'>$2</a></p>")
						.replace(/\n/g, "<br>") +
					`</div>`;
			}

			try {
				const htmlBlob = new Blob([htmlString], { type: "text/html" });
				const textBlob = new Blob([templateString], { type: "text/plain" });
				const item = new ClipboardItem({
					"text/html": htmlBlob,
					"text/plain": textBlob,
				});

				navigator.clipboard.write([item]).then(() => {
					this.copyRichSuccess = true;
					window.setTimeout(() => {
						this.copyRichSuccess = false;
					}, 2000);
				});
			} catch (_err) {
				// Fallback to text copy if ClipboardItem text/html is unsupported
				navigator.clipboard.writeText(templateString).then(() => {
					this.copyRichSuccess = true;
					window.setTimeout(() => {
						this.copyRichSuccess = false;
					}, 2000);
				});
			}
		},
	},

	data() {
		return {
			copyRichSuccess: false,
			selectedFormat: "text", // "text", "html", "md", "json"
		};
	},

	name: "AppMessageModalButton",

	props: {
		burnTime: {
			default: "",
			type: String,
		},
		expiresAt: {
			default: null,
			required: false,
			type: Date,
		},
		secretUrl: {
			required: true,
			type: String,
		},
		securePassword: {
			required: true,
			type: String,
		},
		shortUrl: {
			required: true,
			type: String,
		},
	},
});
</script>
