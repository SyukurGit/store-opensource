/**
 * Digimest Frontend API Configuration
 * Production VPS Endpoint: https://digimest.syukurapi.online
 */

// Global USD Rate & Config Cache
window.digimestUSDRate = parseFloat(sessionStorage.getItem('digimest_usd_rate')) || 18000.0;
window.digimestClientCountry = sessionStorage.getItem('digimest_client_country') || '';

// Immediate Language & Currency Preference Sync & Routing (runs before page rendering)
(function initLocaleAndCurrencyPreference() {
  if (typeof window === 'undefined') return;
  const path = window.location.pathname;
  if (path.startsWith('/admin') || path.startsWith('/api')) return;

  const urlParams = new URLSearchParams(window.location.search);
  const simulateCountry = (urlParams.get('simulate_country') || '').toUpperCase();
  if (simulateCountry) {
    try {
      sessionStorage.setItem('digimest_client_country', simulateCountry);
      window.digimestClientCountry = simulateCountry;
    } catch (e) {}
  }

  const cleanRoute = path.replace(/^\/+/, '').replace(/\.html$/, '');
  const isEnCurrent = cleanRoute === 'en' || cleanRoute.startsWith('en/');
  const savedLang = localStorage.getItem('digimest_lang');
  const savedCur = localStorage.getItem('digimest_currency');

  // Detect Country (Simulation URL -> Session Storage -> Timezone -> Navigator Languages)
  let detectedCountry = simulateCountry || sessionStorage.getItem('digimest_client_country');
  if (!detectedCountry) {
    try {
      const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || '';
      if (/Jakarta|Pontianak|Makassar|Jayapura/i.test(tz)) {
        detectedCountry = 'ID';
      }
    } catch (e) {}
  }
  if (!detectedCountry && typeof navigator !== 'undefined') {
    const langs = navigator.languages || [navigator.language || ''];
    for (const l of langs) {
      if (/^id\b/i.test(l)) {
        detectedCountry = 'ID';
        break;
      }
    }
  }

  // 1. FIRST TIME VISITOR (No saved language preference yet)
  if (!savedLang) {
    const isIndo = detectedCountry === 'ID';
    const initialLang = isIndo ? 'id' : 'en';
    const initialCur = isIndo ? 'IDR' : 'USD';

    try {
      localStorage.setItem('digimest_lang', initialLang);
      document.cookie = `digimest_lang=${initialLang}; path=/; max-age=31536000; SameSite=Lax`;
      if (!savedCur) {
        localStorage.setItem('digimest_currency', initialCur);
      }
    } catch (e) {}

    if (initialLang === 'en' && !isEnCurrent) {
      const baseRoute = (cleanRoute === 'index' || cleanRoute === '') ? '' : cleanRoute;
      const search = window.location.search || '';
      const hash = window.location.hash || '';
      const target = baseRoute ? ('/en/' + baseRoute + search + hash) : ('/en/' + search + hash);
      window.location.replace(target);
      return;
    } else if (initialLang === 'id' && isEnCurrent) {
      const baseRoute = cleanRoute.replace(/^en\/?/, '');
      const search = window.location.search || '';
      const hash = window.location.hash || '';
      const target = baseRoute ? ('/' + baseRoute + search + hash) : ('/' + search + hash);
      window.location.replace(target);
      return;
    }
  } else {
    // 2. RETURNING VISITOR (Respect user's stored language choice!)
    if (isEnCurrent && savedLang !== 'en') {
      try {
        localStorage.setItem('digimest_lang', 'en');
        document.cookie = "digimest_lang=en; path=/; max-age=31536000; SameSite=Lax";
      } catch (e) {}
    } else if (!isEnCurrent && savedLang === 'en') {
      const baseRoute = (cleanRoute === 'index' || cleanRoute === '') ? '' : cleanRoute;
      const search = window.location.search || '';
      const hash = window.location.hash || '';
      const target = baseRoute ? ('/en/' + baseRoute + search + hash) : ('/en/' + search + hash);
      window.location.replace(target);
      return;
    }
  }

  // Ensure default currency is set if not present
  if (!localStorage.getItem('digimest_currency')) {
    const defaultCur = (isEnCurrent || (savedLang === 'en')) ? 'USD' : 'IDR';
    try {
      localStorage.setItem('digimest_currency', defaultCur);
    } catch (e) {}
  }
})();

// Fetch latest dynamic /api/config (usd_rate & country) in background
(function fetchAppConfigAsync() {
  if (typeof window === 'undefined') return;
  const sim = (new URLSearchParams(window.location.search)).get('simulate_country');
  const queryStr = sim ? `?simulate_country=${encodeURIComponent(sim)}` : '';
  const url = (window.digimestApi ? window.digimestApi('/api/config') : '/api/config') + queryStr;

  fetch(url)
    .then(r => r.json())
    .then(data => {
      if (data && (data.success || data.data)) {
        const d = data.data || data;
        if (d.usd_rate) {
          window.digimestUSDRate = parseFloat(d.usd_rate);
          try { sessionStorage.setItem('digimest_usd_rate', d.usd_rate); } catch (e) {}
        }
        if (d.client_country) {
          window.digimestClientCountry = d.client_country;
          try { sessionStorage.setItem('digimest_client_country', d.client_country); } catch (e) {}
        }
        window.dispatchEvent(new CustomEvent('appconfigloaded', { detail: d }));
      }
    })
    .catch(() => {});
})();

(function() {
  const host = window.location.hostname;
  const isLocalDev = host === 'localhost' || host === '127.0.0.1' || host === '0.0.0.0' ||
                     host.endsWith('.test') || host.endsWith('.local') ||
                     window.location.port === '8080' || window.location.port === '3000';
  // Default API VPS: https://digimest.syukurapi.online
  const defaultApi = isLocalDev ? '' : 'https://digimest.syukurapi.online';
  
  window.DIGIMEST_API_BASE = window.DIGIMEST_API_BASE || window.API_BASE || defaultApi;
})();

/**
 * Helper untuk menyusun URL endpoint API secara aman.
 * @param {string} endpoint - Path endpoint (misal: '/api/products' atau 'api/products')
 * @returns {string} Full URL atau relative path
 */
window.digimestApi = function(endpoint) {
  const base = (window.DIGIMEST_API_BASE || '').replace(/\/+$/, '');
  let path = '/' + (endpoint || '').replace(/^\/+/, '');

  // Auto-detect English language context
  const isEn = (typeof document !== 'undefined' && document.documentElement && document.documentElement.lang === 'en') ||
               (typeof window !== 'undefined' && window.location && window.location.pathname.startsWith('/en'));

  if (isEn && (path.startsWith('/api/products') || path.startsWith('/api/public/products'))) {
    if (!path.includes('lang=')) {
      const sep = path.includes('?') ? '&' : '?';
      path = path + sep + 'lang=en';
    }
  }

  return base ? base + path : path;
};

// Current App Build Version
window.STOREKU_BUILD_VERSION = '5.9.6';

// Background Auto-Reload Checker when user returns to tab
(function initVersionWatcher() {
  if (typeof window === 'undefined' || !window.addEventListener) return;
  
  let isChecking = false;
  let lastCheckedTime = 0;
  
  function checkLatestVersion() {
    const now = Date.now();
    // Throttle check to at most once per 60 seconds
    if (isChecking || (now - lastCheckedTime < 60000)) return;
    
    isChecking = true;
    lastCheckedTime = now;
    
    fetch('/version.json?t=' + now, {
      cache: 'no-store',
      headers: { 'Cache-Control': 'no-cache', 'Pragma': 'no-cache' }
    })
      .then(res => {
        if (!res.ok) throw new Error('version check failed');
        return res.json();
      })
      .then(data => {
        if (data && data.version && data.version !== window.STOREKU_BUILD_VERSION) {
          console.log('[Storeku] Versi baru terdeteksi:', data.version, '(Versi saat ini:', window.STOREKU_BUILD_VERSION, ')');
          window.location.reload(true);
        }
      })
      .catch(() => {})
      .finally(() => {
        isChecking = false;
      });
  }

  // Check when tab becomes visible again
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') {
      checkLatestVersion();
    }
  });

  // Check on window focus
  window.addEventListener('focus', checkLatestVersion);
})();
