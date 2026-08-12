import fs from 'fs';
import path from 'path';
import os from 'os';

export function getChromiumExecutablePath(): string | undefined {
  const args = process.argv;

  // 1. Check --with-chrome-bin CLI argument
  const flagIndex = args.indexOf('--with-chrome-bin');
  if (flagIndex !== -1 && args[flagIndex + 1]) {
    const candidate = args[flagIndex + 1];
    if (fs.existsSync(candidate)) return candidate;
  }
  for (const arg of args) {
    if (arg.startsWith('--with-chrome-bin=')) {
      const candidate = arg.split('=')[1];
      if (candidate && fs.existsSync(candidate)) return candidate;
    }
  }

  // 2. Check CHROME_BIN environment variable
  if (process.env.CHROME_BIN && fs.existsSync(process.env.CHROME_BIN)) {
    return process.env.CHROME_BIN;
  }

  // 3. Check OS-specific default paths
  const isWindows = os.platform() === 'win32';
  if (isWindows) {
    const candidates = [
      'D:\\inet\\www\\chromium\\bin\\chrome.exe',
      'D:\\inet\\www\\chromium\\bin\\chromium.exe',
      'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
      'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
    ];
    for (const cand of candidates) {
      if (fs.existsSync(cand)) return cand;
    }
  } else {
    const candidates = [
      '/u01/chromium/bin/chrome',
      '/u01/chromium/bin/chromium',
      '/usr/bin/chromium-browser',
      '/usr/bin/chromium',
      '/usr/bin/google-chrome',
    ];
    for (const cand of candidates) {
      if (fs.existsSync(cand)) return cand;
    }
  }

  return undefined;
}
