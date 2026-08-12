import { mount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import AppMessageModalButton from './message-modal.vue';

describe('AppMessageModalButton Component (UI Unit)', () => {
  const defaultProps = {
    secretUrl: 'http://ots.local.gd/#uuid-1234|passkey123',
    shortUrl: 'http://ots.local.gd/#uuid-1234',
    securePassword: 'passkey123',
    burnTime: '2026-12-31 23:59:59',
  };

  const globalMocks = {
    mocks: {
      $t: (msgKey: string) => msgKey,
    },
  };

  it('renders trigger button correctly', () => {
    const wrapper = mount(AppMessageModalButton, {
      props: defaultProps,
      global: globalMocks,
    });

    const triggerBtn = wrapper.find('button.btn-success');
    expect(triggerBtn.exists()).toBe(true);
    expect(wrapper.find('.modal').exists()).toBe(false);
  });

  it('opens modal on trigger button click', async () => {
    const wrapper = mount(AppMessageModalButton, {
      props: defaultProps,
      global: globalMocks,
    });

    const triggerBtn = wrapper.find('button.btn-success');
    await triggerBtn.trigger('click');

    expect(wrapper.find('.modal').exists()).toBe(true);
    expect(wrapper.find('.modal-title').text()).toContain('title-message-modal');
  });

  it('renders all 4 tabs and template outputs', async () => {
    const wrapper = mount(AppMessageModalButton, {
      props: defaultProps,
      global: globalMocks,
    });

    await wrapper.find('button.btn-success').trigger('click');

    // 1. Full Link tab (Default)
    expect(wrapper.find('textarea').element.value).toContain('http://ots.local.gd/#uuid-1234|passkey123');

    // 2. Switch format to JSON
    const jsonRadio = wrapper.find('#fmtJSON');
    await jsonRadio.setValue();
    expect(wrapper.find('textarea').element.value).toContain('"type": "one_time_secret"');

    // 3. Switch tab to Dual (Link)
    const dualLinkNav = wrapper.findAll('button.nav-link')[1];
    await dualLinkNav.trigger('click');
    expect(wrapper.find('textarea').element.value).toContain('dual_channel_secret_link');

    // 4. Switch tab to Dual (Key)
    const dualKeyNav = wrapper.findAll('button.nav-link')[2];
    await dualKeyNav.trigger('click');
    expect(wrapper.find('textarea').element.value).toContain('dual_channel_decryption_key');

    // 5. Switch tab to Combined Chat
    const combinedNav = wrapper.findAll('button.nav-link')[3];
    await combinedNav.trigger('click');
    expect(wrapper.find('textarea').element.value).toContain('dual_channel_combined_notice');
  });

  it('closes modal when close button is clicked', async () => {
    const wrapper = mount(AppMessageModalButton, {
      props: defaultProps,
      global: globalMocks,
    });

    await wrapper.find('button.btn-success').trigger('click');
    expect(wrapper.find('.modal').exists()).toBe(true);

    const closeBtn = wrapper.find('.modal-header button.btn-close');
    await closeBtn.trigger('click');
    expect(wrapper.find('.modal').exists()).toBe(false);
  });
});
