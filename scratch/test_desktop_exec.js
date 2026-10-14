// Mock DOM to test admin.js execution for reviews.html
const fs = require('fs');

const html = fs.readFileSync('scratch_fe/admin/reviews.html', 'utf8');

// Basic DOM Mock
const elements = {};
const elementTags = [];

function createElement(tag, id = '') {
  const el = {
    tagName: tag.toUpperCase(),
    id: id,
    style: {},
    classList: {
      add: () => {},
      remove: () => {},
      contains: () => false,
      toggle: () => {}
    },
    dataset: {},
    children: [],
    addEventListener: (evt, fn) => {
      // console.log(`[EVENT] added ${evt} on #${id || tag}`);
    },
    removeEventListener: () => {},
    setAttribute: () => {},
    getAttribute: () => '',
    value: '',
    textContent: '',
    innerHTML: '',
    reset: () => {},
    click: () => {}
  };
  return el;
}

// Extract IDs from reviews.html
const idMatches = html.matchAll(/id="([^"]+)"/g);
for (const m of idMatches) {
  elements[m[1]] = createElement('div', m[1]);
}

global.window = {
  location: {
    hostname: 'localhost',
    pathname: '/admin/reviews',
    search: '',
    port: ''
  },
  localStorage: {
    getItem: (k) => 'test-token',
    setItem: () => {},
    removeItem: () => {}
  },
  addEventListener: () => {},
  removeEventListener: () => {}
};

global.document = {
  getElementById: (id) => {
    if (!elements[id]) {
      // console.log('[DOM] getElementById null for:', id);
      return null;
    }
    return elements[id];
  },
  querySelectorAll: (sel) => [],
  querySelector: (sel) => null,
  addEventListener: (evt, fn) => {
    if (evt === 'DOMContentLoaded') {
      console.log('[DOM] Triggering DOMContentLoaded...');
      try {
        fn();
        console.log('[DOM] DOMContentLoaded finished without error!');
      } catch (err) {
        console.error('[DOM ERROR in DOMContentLoaded]:', err);
      }
    }
  }
};

global.localStorage = global.window.localStorage;
global.fetch = async (url) => {
  console.log('[FETCH] calling:', url);
  return {
    ok: true,
    status: 200,
    json: async () => ({ success: true, data: [] })
  };
};

// Run api-config.js
require('../scratch_fe/js/api-config.js');

// Run admin.js
console.log('[TEST] Evaluating admin.js...');
try {
  require('../scratch_fe/js/admin.js');
  console.log('[TEST] admin.js loaded successfully!');
} catch (e) {
  console.error('[TEST SCRIPT ERROR]:', e);
}
