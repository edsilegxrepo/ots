# Detailed UI / UX Analysis & Enhancement Recommendations (UI.md)

Target Release: v1.50.0

This document details the user interface, accessibility, visual presentation, and user experience enhancement recommendations for the Vue 3 Single Page Application (src/app.vue, src/components/create.vue, src/components/display-url.vue, src/components/secret-display.vue, src/components/navbar.vue).

---

## Executive Summary

Following a detailed audit of the Vue 3 frontend components, key UI and UX refinements have been identified to streamline secret creation, mobile touch interaction feedback, passphrase generation, receiver burn warnings, and repository link reconciliation.

---

## 1. Repository Link Reconciliation (src/app.vue)

- Current Observation: Line 42 in src/app.vue links to the legacy repository path https://github.com/Luzifer/ots.
- Recommendation: Update the href attribute to https://github.com/edsilegxrepo/ots to match the updated repository namespace.

---

## 2. Secret Link Copying & Mobile Usability Refinements (src/components/create.vue & src/components/clipboard-button.vue)

### Existing Implementation Audit
Upon inspecting src/components/display-url.vue (lines 52-73), Dual-Channel Delivery is already 100% implemented with 2 distinct copy buttons:
1. Short Link (without key) copy button (`:content="shortUrl"`).
2. Decryption Key copy button (`:content="securePassword"`).

### Proposed Mobile UX Refinement: Transient Button-Level Feedback
- Existing Setup: Copy buttons currently rely on desktop tooltips (`title="Copy to clipboard"`), which fail to display on mobile touchscreens.
- Proposed Refinement: Update src/components/clipboard-button.vue to provide immediate button-level state switching (e.g. icon temporarily changes to a checkmark and label to "Copied!" for 2,000ms before reverting), giving clear visual confirmation on touch devices.

---

## 3. Passphrase Length & Generator Upgrade (src/components/create.vue)

- Current State: The secret creation view auto-generates a 20-character CSPRNG key (~119 bits of entropy).
- Recommendation:
  - Upgrade default auto-generated secret key length from 20 characters to 32 characters (`passwordLength = 32`), boosting entropy to 190.5 bits and directly aligning key strength with the 256-bit AES-256-GCM cipher engine.
  - Add an inline 1-Click Password Generator ("Gen Pass") button directly in the creation form for custom passphrases.
  - Display an inline Entropy / Strength Indicator (Weak / Good / Strong) when custom passphrases are entered.

---

## 4. Destruction Alert & Plaintext Save Shortcuts (src/components/secret-display.vue)

### Existing Implementation Audit
Upon inspecting src/components/secret-display.vue (lines 18-24), a full Zip bundle download button (`downloadBundle`) is already implemented.

### Proposed Refinement: Burn Alert Banner & Direct Text Save
- Burn Alert Banner: Render a prominent warning banner when a secret is displayed with zero remaining reads:
  > Burned Secret Alert: This secret has been permanently deleted from server memory. Copy or save your content now before closing this tab.
- Plaintext Save (.txt): Add a direct "Save as Text (.txt)" shortcut alongside the Zip bundle downloader for receivers who only need to save the text payload without unzipping an archive.

---

## 5. Monospace Code Readability & Dark Mode Polish

- Monospace Pre Blocks: Ensure code snippets and secret text areas use font-family: var(--bs-font-monospace) with white-space: pre-wrap and subtle background tinting (var(--bs-tertiary-bg)) to prevent line clipping for long SSH keys, JSON blocks, or API tokens.
- Badge Styling: Continue using the monospaced extension badges (.png, .pdf, .zip) added under Attach Files: with dark-mode high-contrast borders.

---

## 6. Summary Table of Recommended UI Enhancements

| Component | Target Area | Recommended UI / UX Enhancement | Benefit |
| :--- | :--- | :--- | :--- |
| src/app.vue | Footer | Update GitHub link to github.com/edsilegxrepo/ots | Correct repository reference |
| src/components/clipboard-button.vue | Clipboard Buttons | Add transient "Copied!" button state for 2,000ms | Immediate feedback on mobile touchscreens |
| src/components/create.vue | Secret Creation | Upgrade auto-key to 32 chars, add "Gen Pass" button & Strength Meter | Higher passphrase entropy matching AES-256 |
| src/components/secret-display.vue | Secret Receiver | Add Burn Warning Banner & Direct "Save as Text (.txt)" shortcut | Prevents accidental data loss |
| src/components/navbar.vue | Header | Preserve High-Contrast Auto / Dark / Light theme toggle | Enhanced night-mode accessibility |
