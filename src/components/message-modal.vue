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
                ><i class="fas fa-brackets-curly me-1" /> JSON</label>
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
                <div class="d-flex justify-content-end">
                  <app-clipboard-button
                    :content="fullLinkTemplate"
                    title="Copy Full Link Message"
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
                <div class="d-flex justify-content-end">
                  <app-clipboard-button
                    :content="dualLinkTemplate"
                    title="Copy Dual Link Message"
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
                <div class="d-flex justify-content-end">
                  <app-clipboard-button
                    :content="dualKeyTemplate"
                    title="Copy Decryption Key Message"
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
                <div class="d-flex justify-content-end">
                  <app-clipboard-button
                    :content="combinedChatTemplate"
                    title="Copy Combined Chat Notice"
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
		combinedChatTemplate(): string {
			if (this.selectedFormat === "json") {
				return JSON.stringify(
					{
						burn_on_read: true,
						decryption_key: this.securePassword,
						short_url: this.shortUrl,
						type: "dual_channel_combined_notice",
					},
					null,
					2,
				);
			}

			if (this.selectedFormat === "html") {
				return `<div style="font-family: Arial, sans-serif; border: 1px solid #0dcaf0; border-radius: 6px; padding: 15px; background-color: #f8f9fa;">
  <h4 style="color: #055160; margin-top: 0;">DUAL-CHANNEL SECURE DELIVERY NOTICE</h4>
  <p style="margin-bottom: 5px;"><strong>Link (Channel 1):</strong> <a href="${this.shortUrl}">${this.shortUrl}</a></p>
  <p style="margin-bottom: 10px;"><strong>Key (Channel 2):</strong> <code>${this.securePassword}</code></p>
  <p style="font-size: 12px; color: #6c757d; margin: 0;">NOTICE: Opening the link decrypts and burns the secret immediately.</p>
</div>`;
			}

			if (this.selectedFormat === "md") {
				return `### DUAL-CHANNEL SECURE DELIVERY NOTICE\n\n- **Link (Channel 1):** ${this.shortUrl}\n- **Key  (Channel 2):** \`${this.securePassword}\` \n\n*NOTICE: Opening the link decrypts and burns the secret immediately.*`;
			}

			// Plain Text Default
			return `\`\`\`\n===================================================================\n             DUAL-CHANNEL SECURE DELIVERY NOTICE\n===================================================================\n\n  Link (Channel 1): ${this.shortUrl}\n  Key  (Channel 2): ${this.securePassword}\n\nNOTICE: Opening the link decrypts and burns the secret immediately.\n===================================================================\n\`\`\``;
		},

		dualKeyTemplate(): string {
			if (this.selectedFormat === "json") {
				return JSON.stringify(
					{
						decryption_key: this.securePassword,
						instructions: "Paste this key when prompted after opening your secret link.",
						type: "dual_channel_decryption_key",
					},
					null,
					2,
				);
			}

			if (this.selectedFormat === "html") {
				return `<div style="font-family: Arial, sans-serif; border: 1px solid #ffc107; border-radius: 6px; padding: 15px; background-color: #fff3cd;">
  <h3 style="color: #664d03; margin-top: 0;">DECRYPTION KEY TRANSMISSION (PART 2 OF 2)</h3>
  <p>Hello,</p>
  <p>Here is your decryption key for the secret link sent via email:</p>
  <div style="background: #ffffff; border: 1px solid #ffe69c; padding: 10px; font-family: monospace; font-size: 14px; margin: 10px 0;">
    <strong>DECRYPTION KEY:</strong><br>${this.securePassword}
  </div>
  <p style="font-size: 12px; color: #664d03;">INSTRUCTIONS: Paste this key when prompted after opening your secret link.</p>
</div>`;
			}

			if (this.selectedFormat === "md") {
				return `### DECRYPTION KEY TRANSMISSION (PART 2 OF 2)\n\nHello,\n\nHere is your decryption key for the secret link sent via email:\n\n> **DECRYPTION KEY:**\n> \`${this.securePassword}\` \n\n**INSTRUCTIONS:** Paste this key when prompted after opening your secret link.`;
			}

			// Plain Text Default
			return `===================================================================\n            DECRYPTION KEY TRANSMISSION (PART 2 OF 2)\n===================================================================\n\nHello,\n\nHere is your decryption key for the secret link sent via email:\n\n-------------------------------------------------------------------\nDECRYPTION KEY:\n${this.securePassword}\n-------------------------------------------------------------------\n\nINSTRUCTIONS:\nPaste this key when prompted after opening your secret link.\n\n===================================================================`;
		},

		dualLinkTemplate(): string {
			let expiryTxt = "";
			if (this.burnTime) {
				expiryTxt = `\n3. Expiration: ${this.burnTime}.`;
			}

			if (this.selectedFormat === "json") {
				const obj: Record<string, unknown> = {
					burn_on_read: true,
					decryption_key_channel: "separate_channel_required",
					short_url: this.shortUrl,
					type: "dual_channel_secret_link",
				};
				if (this.burnTime) {
					obj.expires_at = this.burnTime;
				}
				return JSON.stringify(obj, null, 2);
			}

			if (this.selectedFormat === "html") {
				return `<div style="font-family: Arial, sans-serif; border: 1px solid #198754; border-radius: 6px; padding: 15px; background-color: #ffffff;">
  <h3 style="color: #0f5132; margin-top: 0;">DUAL-CHANNEL SECRET TRANSMISSION (PART 1 OF 2)</h3>
  <p>Hello,</p>
  <p>A secure, encrypted one-time secret has been created for you. For enhanced security, the decryption key is delivered via a separate channel (SMS, Slack, or Teams).</p>
  <div style="background-color: #f8f9fa; border-left: 4px solid #198754; padding: 10px; margin: 10px 0; word-break: break-all;">
    <strong>SECRET LINK (Without Decryption Key):</strong><br>
    <a href="${this.shortUrl}" style="color: #0d6efd;">${this.shortUrl}</a>
  </div>
  <p style="font-size: 13px; color: #6c757d;">
    <strong>INSTRUCTIONS:</strong><br>
    1. Obtain the Decryption Key from your second communication channel.<br>
    2. Accessing the link burns (deletes) the secret permanently.${this.burnTime ? '<br>3. Expiration: ' + this.burnTime + '.' : ''}
  </p>
</div>`;
			}

			if (this.selectedFormat === "md") {
				return `### DUAL-CHANNEL SECRET TRANSMISSION (PART 1 OF 2)\n\nHello,\n\nA secure, encrypted one-time secret has been created for you.\nFor enhanced security, the decryption key is delivered via a separate channel (SMS, Slack, or Teams).\n\n> **SECRET LINK (Without Decryption Key):**\n> ${this.shortUrl}\n\n**INSTRUCTIONS:**\n1. Obtain the Decryption Key from your second communication channel.\n2. Accessing the link burns (deletes) the secret permanently.${expiryTxt}`;
			}

			// Plain Text Default
			return `===================================================================\n            DUAL-CHANNEL SECRET TRANSMISSION (PART 1 OF 2)\n===================================================================\n\nHello,\n\nA secure, encrypted one-time secret has been created for you.\nFor enhanced security, the decryption key is delivered via a \nseparate channel (SMS, Slack, or Teams).\n\n-------------------------------------------------------------------\nSECRET LINK (Without Decryption Key):\n${this.shortUrl}\n-------------------------------------------------------------------\n\nINSTRUCTIONS:\n1. Obtain the Decryption Key from your second communication channel.\n2. Accessing the link burns (deletes) the secret permanently.${expiryTxt}\n\n===================================================================`;
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
			let expiryTxt = "";
			if (this.burnTime) {
				expiryTxt = `\n3. Expiration: ${this.burnTime} (if not viewed before).`;
			}

			if (this.selectedFormat === "json") {
				const obj: Record<string, unknown> = {
					burn_on_read: true,
					instructions: "Accessing this URL decrypts the payload and PERMANENTLY BURNS (deletes) the secret from the server.",
					secret_url: this.secretUrl,
					type: "one_time_secret",
				};
				if (this.burnTime) {
					obj.expires_at = this.burnTime;
				}
				return JSON.stringify(obj, null, 2);
			}

			if (this.selectedFormat === "html") {
				return `<div style="font-family: Arial, sans-serif; border: 1px solid #198754; border-radius: 6px; padding: 15px; background-color: #ffffff;">
  <h3 style="color: #0f5132; margin-top: 0;">CONFIDENTIAL ONE-TIME SECRET</h3>
  <p>Hello,</p>
  <p>A secure, encrypted one-time secret has been generated for you.</p>
  <div style="background-color: #f8f9fa; border-left: 4px solid #198754; padding: 10px; margin: 10px 0; word-break: break-all;">
    <strong>SECRET URL:</strong><br>
    <a href="${this.secretUrl}" style="color: #0d6efd;">${this.secretUrl}</a>
  </div>
  <p style="font-size: 13px; color: #6c757d;">
    <strong>IMPORTANT INSTRUCTIONS:</strong><br>
    1. Accessing this URL decrypts the payload and <strong>PERMANENTLY BURNS (deletes)</strong> the secret.<br>
    2. Please copy or store the content immediately upon opening.${this.burnTime ? '<br>3. Expiration: ' + this.burnTime + ' (if not viewed before).' : ''}
  </p>
</div>`;
			}

			if (this.selectedFormat === "md") {
				return `### CONFIDENTIAL ONE-TIME SECRET\n\nHello,\n\nA secure, encrypted one-time secret has been generated for you.\n\n> **SECRET URL:**\n> ${this.secretUrl}\n\n**IMPORTANT INSTRUCTIONS:**\n1. Accessing this URL decrypts the payload and **PERMANENTLY BURNS (deletes)** the secret from the server.\n2. Please copy or store the content immediately upon opening.${expiryTxt}`;
			}

			// Plain Text Default
			return `===================================================================\n                  CONFIDENTIAL ONE-TIME SECRET\n===================================================================\n\nHello,\n\nA secure, encrypted one-time secret has been generated for you.\n\n-------------------------------------------------------------------\nSECRET URL:\n${this.secretUrl}\n-------------------------------------------------------------------\n\nIMPORTANT INSTRUCTIONS:\n1. Accessing this URL decrypts the payload and PERMANENTLY BURNS\n   (deletes) the secret from the server.\n2. Please copy or store the content immediately upon opening.${expiryTxt}\n\n===================================================================`;
		},
	},

	data() {
		return {
			selectedFormat: "text", // "text", "html", "md", "json"
		};
	},

	name: "AppMessageModalButton",

	props: {
		burnTime: {
			default: "",
			type: String,
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
