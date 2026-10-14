const fs = require('fs');

const html = fs.readFileSync('scratch_fe/admin/reviews.html', 'utf8');
const adminJs = fs.readFileSync('scratch_fe/js/admin.js', 'utf8');

// Simple DOM Mock
const elements = {};
// Parse all elements with id
const idRegex = /id=["']([^"']+)["']/g;
let m;
while ((m = idRegex.exec(html)) !== null) {
  elements[m[1]] = {
    id: m[1],
    style: {},
    classList: {
      add: () => {},
      remove: () => {},
      toggle: () => {},
      contains: () => false
    },
    addEventListener: () => {},
    value: '',
    textContent: '',
    innerHTML: '',
    checked: false
  };
}

const mockDocument = {
  getElementById: (id) => elements[id] || null,
  querySelectorAll: (sel) => [],
  addEventListener: (event, cb) => {
    if (event === 'DOMContentLoaded') {
      console.log('Running DOMContentLoaded callback...');
      try {
        cb();
        console.log('DOMContentLoaded executed successfully!');
        setTimeout(() => {
          console.log('etalaseTableBody innerHTML:', elements['etalaseTableBody']?.innerHTML);
        }, 100);
      } catch (err) {
        console.error('ERROR during DOMContentLoaded:', err);
      }
    }
  }
};

const mockWindow = {
  location: { pathname: '/admin/reviews', search: '', hostname: 'localhost' },
  addEventListener: () => {}
};

// Test by evaluating admin.js in mock environment
try {
  const fn = new Function('document', 'window', 'localStorage', 'fetch', 'showToast', adminJs);
  fn(mockDocument, mockWindow, { getItem: () => 'token', removeItem: () => {}, setItem: () => {} }, () => Promise.resolve({ ok: true, json: () => Promise.resolve({ success: true, data: [] }) }), () => {});
} catch (e) {
  console.error('GLOBAL EVAL ERROR:', e);
}
