<template>
  <div id="app">
    <app-navbar
      v-model:theme="theme"
      @navigate="navigate"
    />

    <div class="container mt-4">
      <div
        v-if="error"
        class="row justify-content-center"
      >
        <div class="col-12 col-md-8">
          <!-- Safe: Error messages sanitized internally before assignment -->
          <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
          <div
            class="alert alert-danger"
            role="alert"
            v-html="error"
          />
        </div>
      </div>

      <div class="row">
        <div class="col">
          <router-view
            @error="displayError"
            @navigate="navigate"
          />
        </div>
      </div>

      <div
        class="row mt-4"
      >
        <div class="col form-text text-center">
          <span
            v-if="!customize.disablePoweredBy"
            class="mx-2"
          >
            {{ $t('text-powered-by') }}
            <a href="https://github.com/Luzifer/ots"><i class="fab fa-github" /> OTS</a>
            {{ version }}
          </span>
          <span
            v-for="link in customize.footerLinks"
            :key="link.url"
            class="mx-2"
          >
            <a :href="link.url">{{ link.name }}</a>
          </span>
        </div>
      </div>
    </div>

    <!-- Explanation Modal -->
    <div
      id="explanationModal"
      class="modal fade"
      tabindex="-1"
      aria-labelledby="explanationModalLabel"
      aria-hidden="true"
    >
      <div class="modal-dialog modal-dialog-centered modal-lg">
        <div class="modal-content">
          <div class="modal-header bg-primary-subtle">
            <!-- Safe: Trusted internal translation string from i18n.yaml -->
            <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html -->
            <h5
              id="explanationModalLabel"
              class="modal-title"
              v-html="$t('title-explanation')"
            />
            <button
              type="button"
              class="btn-close"
              data-bs-dismiss="modal"
              aria-label="Close"
            />
          </div>
          <div class="modal-body">
            <ul>
              <li
                v-for="(explanation, idx) in $tm('items-explanation')"
                :key="`idx${idx}`"
              >
                {{ explanation }}
              </li>
            </ul>
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
import { isNavigationFailure, NavigationFailureType } from "vue-router";
import AppNavbar from "./components/navbar.vue";

export default defineComponent({
	components: { AppNavbar },

	computed: {
		isSecureEnvironment(): boolean {
			return Boolean(window.crypto.subtle);
		},

		version(): string {
			return window.version;
		},
	},

	created() {
		this.navigate("/");
	},

	data() {
		return {
			customize: {} as any,
			error: "" as string | null,
			theme: "auto",
		};
	},

	methods: {
		displayError(error: string | null) {
			this.error = error;
		},

		// hashLoad reacts on a changed window hash an starts the diplaying of the secret
		hashLoad() {
			const hash = decodeURIComponent(window.location.hash);
			if (hash.length === 0) {
				return;
			}

			const parts = hash.substring(1).split("|");
			const secretId = parts[0];
			let securePassword = null as string | null;

			if (parts.length === 2) {
				securePassword = parts[1];
			}

			this.navigate({
				path: "/secret",
				query: {
					secretId,
					securePassword,
				},
			});
		},

		navigate(to: string | any): void {
			this.error = "";
			this.$router.replace(to).catch((err) => {
				if (isNavigationFailure(err, NavigationFailureType.duplicated)) {
					// Hide duplicate nav errors
					return;
				}
				throw err;
			});
		},
	},

	// Trigger initialization functions
	mounted() {
		this.customize = window.OTSCustomize;

		window.onhashchange = this.hashLoad;
		this.hashLoad();

		if (!this.isSecureEnvironment) {
			this.error = this.$t("alert-insecure-environment");
		}

		this.theme = window.getThemeFromStorage();
		window
			.matchMedia("(prefers-color-scheme: light)")
			.addEventListener("change", () => {
				window.refreshTheme();
			});
	},

	name: "App",

	watch: {
		theme(to): void {
			window.setTheme(to);
		},
	},
});
</script>
