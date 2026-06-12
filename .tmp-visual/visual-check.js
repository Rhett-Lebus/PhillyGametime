const { chromium } = require('playwright');

const baseURL = 'http://localhost:8090';
const chromePath = 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';

const cases = [
  { name: 'world-cup-desktop', path: '/world-cup', width: 1440, height: 1100 },
  { name: 'world-cup-mobile', path: '/world-cup', width: 390, height: 1200 },
  { name: 'scores-desktop', path: '/scores', width: 1440, height: 1100 },
  { name: 'scores-mobile', path: '/scores', width: 390, height: 1200 },
];

(async () => {
  const browser = await chromium.launch({
    executablePath: chromePath,
    headless: true,
  });
  try {
    for (const item of cases) {
      const page = await browser.newPage({ viewport: { width: item.width, height: item.height } });
      await page.goto(`${baseURL}${item.path}`, { waitUntil: 'networkidle' });
      await page.screenshot({ path: `.tmp-visual/${item.name}.png`, fullPage: true });
      await page.close();
      console.log(`${item.name}.png`);
    }
  } finally {
    await browser.close();
  }
})();
