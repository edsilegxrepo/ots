<!--
  AppMessageModalButton Component

  Objectives:
  - Renders the [ Generate Message ] modal trigger button and interactive enterprise message modal.
  - Supports 4 template modes (Full Link, Dual Link, Dual Key, Combined Chat) and 4 output formats (Text, HTML, Markdown, JSON).
  - Uses 100% reactive Vue state (showModal & activeTab) for reliable rendering without external jQuery/Bootstrap event binding.
-->
<template>
  <div>
    <!-- Trigger Button -->
    <button
      class="btn btn-success btn-sm text-white shadow-sm fw-semibold"
      type="button"
      :title="$t('tooltip-generate-message')"
      @click="showModal = true"
    >
      <i class="fas fa-envelope-open-text fa-fw me-1" />
      <span class="d-none d-md-inline">{{ $t('btn-generate-message') }}</span>
    </button>

    <!-- Enterprise Message Modal -->
    <div
      v-if="showModal"
      class="modal fade show d-block"
      tabindex="-1"
      style="background-color: rgba(0, 0, 0, 0.6);"
      @click.self="showModal = false"
    >
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content shadow-lg border-info-subtle">
          <div class="modal-header bg-info-subtle py-2">
            <h5 class="modal-title fw-bold text-info-emphasis mb-0 d-flex align-items-center">
              <i class="fas fa-file-signature me-2" />{{ $t('title-message-modal') }}
            </h5>
            <button
              type="button"
              class="btn-close"
              aria-label="Close"
              @click="showModal = false"
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
                    :class="['nav-link', { active: activeTab === 'full-link' }]"
                    type="button"
                    role="tab"
                    @click="activeTab = 'full-link'"
                  >
                    <i class="fas fa-link me-1" /> Full Link
                  </button>
                </li>
                <li
                  class="nav-item"
                  role="presentation"
                >
                  <button
                    :class="['nav-link', { active: activeTab === 'dual-link' }]"
                    type="button"
                    role="tab"
                    @click="activeTab = 'dual-link'"
                  >
                    <i class="fas fa-shield-halved me-1" /> Dual (Link)
                  </button>
                </li>
                <li
                  class="nav-item"
                  role="presentation"
                >
                  <button
                    :class="['nav-link', { active: activeTab === 'dual-key' }]"
                    type="button"
                    role="tab"
                    @click="activeTab = 'dual-key'"
                  >
                    <i class="fas fa-key me-1" /> Dual (Key)
                  </button>
                </li>
                <li
                  class="nav-item"
                  role="presentation"
                >
                  <button
                    :class="['nav-link', { active: activeTab === 'dual-combined' }]"
                    type="button"
                    role="tab"
                    @click="activeTab = 'dual-combined'"
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
                v-if="activeTab === 'full-link'"
                class="tab-pane active show"
                role="tabpanel"
              >
                <p class="small text-secondary mb-2">
                  Complete delivery template containing full secret decryption URL. (Format: {{ formatLabel }}).
                </p>
                <div
                  v-if="selectedFormat !== 'html' || showHTMLCode"
                  class="position-relative mb-2"
                >
                  <textarea
                    class="form-control font-monospace small"
                    rows="10"
                    readonly
                    :value="fullLinkTemplate"
                  />
                </div>
                <div
                  v-if="selectedFormat === 'html'"
                  id="preview-tab1"
                  class="rendered-html-preview border rounded p-3 mb-2 bg-white shadow-sm"
                  v-html="fullLinkTemplate"
                />
                <div class="d-flex justify-content-end gap-2">
                  <button
                    v-if="selectedFormat === 'html'"
                    type="button"
                    class="btn btn-outline-danger shadow-sm"
                    @click="showHTMLCode = !showHTMLCode"
                  >
                    <i :class="showHTMLCode ? 'fas fa-eye me-1' : 'fas fa-code me-1'" />
                    {{ showHTMLCode ? 'Hide Code' : 'Show Code' }}
                  </button>
                  <button
                    v-if="selectedFormat === 'html'"
                    type="button"
                    class="btn shadow-sm"
                    :class="copyRichSuccess ? 'btn-success text-white' : 'btn-outline-success'"
                    title="Copy formatted rich text to paste rendered cards into Outlook, Teams, Word, or Slack"
                    @click="copyFormattedHTML(fullLinkTemplate, 'preview-tab1')"
                  >
                    <i :class="copyRichSuccess ? 'fas fa-check me-1' : 'fas fa-wand-magic-sparkles me-1'" />
                    {{ copyRichSuccess ? 'Copied Formatted HTML!' : 'Copy Formatted HTML' }}
                  </button>
                  <app-clipboard-button
                    :content="fullLinkTemplate"
                    title="Copy Full Link Message"
                    :show-label="true"
                    label-text="Copy Message"
                  />
                </div>
              </div>

              <!-- Tab 2: Dual Channel Link Message -->
              <div
                v-if="activeTab === 'dual-link'"
                class="tab-pane active show"
                role="tabpanel"
              >
                <p class="small text-secondary mb-2">
                  Channel 1 Template (Email/Ticket): Short link without key. (Format: {{ formatLabel }}).
                </p>
                <div
                  v-if="selectedFormat !== 'html' || showHTMLCode"
                  class="position-relative mb-2"
                >
                  <textarea
                    class="form-control font-monospace small"
                    rows="10"
                    readonly
                    :value="dualLinkTemplate"
                  />
                </div>
                <div
                  v-if="selectedFormat === 'html'"
                  id="preview-tab2"
                  class="rendered-html-preview border rounded p-3 mb-2 bg-white shadow-sm"
                  v-html="dualLinkTemplate"
                />
                <div class="d-flex justify-content-end gap-2">
                  <button
                    v-if="selectedFormat === 'html'"
                    type="button"
                    class="btn btn-outline-danger shadow-sm"
                    @click="showHTMLCode = !showHTMLCode"
                  >
                    <i :class="showHTMLCode ? 'fas fa-eye me-1' : 'fas fa-code me-1'" />
                    {{ showHTMLCode ? 'Hide Code' : 'Show Code' }}
                  </button>
                  <button
                    v-if="selectedFormat === 'html'"
                    type="button"
                    class="btn shadow-sm"
                    :class="copyRichSuccess ? 'btn-success text-white' : 'btn-outline-success'"
                    title="Copy formatted rich text to paste rendered cards into Outlook, Teams, Word, or Slack"
                    @click="copyFormattedHTML(dualLinkTemplate, 'preview-tab2')"
                  >
                    <i :class="copyRichSuccess ? 'fas fa-check me-1' : 'fas fa-wand-magic-sparkles me-1'" />
                    {{ copyRichSuccess ? 'Copied Formatted HTML!' : 'Copy Formatted HTML' }}
                  </button>
                  <app-clipboard-button
                    :content="dualLinkTemplate"
                    title="Copy Dual Link Message"
                    :show-label="true"
                    label-text="Copy Link Message"
                  />
                </div>
              </div>

              <!-- Tab 3: Dual Channel Key Message -->
              <div
                v-if="activeTab === 'dual-key'"
                class="tab-pane active show"
                role="tabpanel"
              >
                <p class="small text-secondary mb-2">
                  Channel 2 Template (SMS/Teams/Slack): Decryption key only. (Format: {{ formatLabel }}).
                </p>
                <div
                  v-if="selectedFormat !== 'html' || showHTMLCode"
                  class="position-relative mb-2"
                >
                  <textarea
                    class="form-control font-monospace small"
                    rows="8"
                    readonly
                    :value="dualKeyTemplate"
                  />
                </div>
                <div
                  v-if="selectedFormat === 'html'"
                  id="preview-tab3"
                  class="rendered-html-preview border rounded p-3 mb-2 bg-white shadow-sm"
                  v-html="dualKeyTemplate"
                />
                <div class="d-flex justify-content-end gap-2">
                  <button
                    v-if="selectedFormat === 'html'"
                    type="button"
                    class="btn btn-outline-danger shadow-sm"
                    @click="showHTMLCode = !showHTMLCode"
                  >
                    <i :class="showHTMLCode ? 'fas fa-eye me-1' : 'fas fa-code me-1'" />
                    {{ showHTMLCode ? 'Hide Code' : 'Show Code' }}
                  </button>
                  <button
                    v-if="selectedFormat === 'html'"
                    type="button"
                    class="btn shadow-sm"
                    :class="copyRichSuccess ? 'btn-success text-white' : 'btn-outline-success'"
                    title="Copy formatted rich text to paste rendered cards into Outlook, Teams, Word, or Slack"
                    @click="copyFormattedHTML(dualKeyTemplate, 'preview-tab3')"
                  >
                    <i :class="copyRichSuccess ? 'fas fa-check me-1' : 'fas fa-wand-magic-sparkles me-1'" />
                    {{ copyRichSuccess ? 'Copied Formatted HTML!' : 'Copy Formatted HTML' }}
                  </button>
                  <app-clipboard-button
                    :content="dualKeyTemplate"
                    title="Copy Decryption Key Message"
                    :show-label="true"
                    label-text="Copy Key Message"
                  />
                </div>
              </div>

              <!-- Tab 4: Combined Chat Notice -->
              <div
                v-if="activeTab === 'dual-combined'"
                class="tab-pane active show"
                role="tabpanel"
              >
                <p class="small text-secondary mb-2">
                  Combined notice formatted for instant pasting into internal chat tools or API pipelines. (Format: {{ formatLabel }}).
                </p>
                <div
                  v-if="selectedFormat !== 'html' || showHTMLCode"
                  class="position-relative mb-2"
                >
                  <textarea
                    class="form-control font-monospace small"
                    rows="8"
                    readonly
                    :value="combinedChatTemplate"
                  />
                </div>
                <div
                  v-if="selectedFormat === 'html'"
                  id="preview-tab4"
                  class="rendered-html-preview border rounded p-3 mb-2 bg-white shadow-sm"
                  v-html="combinedChatTemplate"
                />
                <div class="d-flex justify-content-end gap-2">
                  <button
                    v-if="selectedFormat === 'html'"
                    type="button"
                    class="btn btn-outline-danger shadow-sm"
                    @click="showHTMLCode = !showHTMLCode"
                  >
                    <i :class="showHTMLCode ? 'fas fa-eye me-1' : 'fas fa-code me-1'" />
                    {{ showHTMLCode ? 'Hide Code' : 'Show Code' }}
                  </button>
                  <button
                    v-if="selectedFormat === 'html'"
                    type="button"
                    class="btn shadow-sm"
                    :class="copyRichSuccess ? 'btn-success text-white' : 'btn-outline-success'"
                    title="Copy formatted rich text to paste rendered cards into Outlook, Teams, Word, or Slack"
                    @click="copyFormattedHTML(combinedChatTemplate, 'preview-tab4')"
                  >
                    <i :class="copyRichSuccess ? 'fas fa-check me-1' : 'fas fa-wand-magic-sparkles me-1'" />
                    {{ copyRichSuccess ? 'Copied Formatted HTML!' : 'Copy Formatted HTML' }}
                  </button>
                  <app-clipboard-button
                    :content="combinedChatTemplate"
                    title="Copy Combined Chat Notice"
                    :show-label="true"
                    label-text="Copy Combined Notice"
                  />
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer py-2">
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              @click="showModal = false"
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
	},

	data() {
		return {
			activeTab: "full-link",
			copyRichSuccess: false,
			copyRichTimeout: null as number | null,
			selectedFormat: "text", // "text", "html", "md", "json"
			showHTMLCode: false,
			showModal: false,
		};
	},

	methods: {
		async copyFormattedHTML(templateText: string, elementId?: string): Promise<void> {
			let success = false;

			if (elementId) {
				const targetEl = document.getElementById(elementId);
				if (targetEl) {
					const selection = window.getSelection();
					const range = document.createRange();
					range.selectNodeContents(targetEl);
					if (selection) {
						selection.removeAllRanges();
						selection.addRange(range);
					}
					try {
						success = document.execCommand("copy");
					} catch (_e) {
						success = false;
					}
					if (selection) {
						selection.removeAllRanges();
					}
				}
			}

			if (!success) {
				try {
					const htmlBlob = new Blob([templateText], { type: "text/html" });
					const textBlob = new Blob([templateText], { type: "text/plain" });
					await navigator.clipboard.write([
						new ClipboardItem({
							"text/html": htmlBlob,
							"text/plain": textBlob,
						}),
					]);
					success = true;
				} catch (_err) {
					const listener = (e: ClipboardEvent) => {
						e.preventDefault();
						if (e.clipboardData) {
							e.clipboardData.setData("text/html", templateText);
							e.clipboardData.setData("text/plain", templateText);
						}
					};

					document.addEventListener("copy", listener);
					try {
						document.execCommand("copy");
						success = true;
					} catch (_e2) {
						navigator.clipboard.writeText(templateText);
					} finally {
						document.removeEventListener("copy", listener);
					}
				}
			}

			if (success) {
				this.copyRichSuccess = true;
				if (this.copyRichTimeout) {
					window.clearTimeout(this.copyRichTimeout);
				}
				this.copyRichTimeout = window.setTimeout(() => {
					this.copyRichSuccess = false;
				}, 2500);
			}
		},
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
