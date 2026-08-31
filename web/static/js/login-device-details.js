(() => {
  const form = document.querySelector('form[data-login-device-details]');
  if (!form) return;

  const field = document.createElement('input');
  field.type = 'hidden';
  field.name = 'client_details';
  form.appendChild(field);

  const details = {
    languages: navigator.languages || (navigator.language ? [navigator.language] : []),
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || '',
    screen: `${screen.width} × ${screen.height}`,
    viewport: `${window.innerWidth} × ${window.innerHeight}`,
    color_depth: screen.colorDepth || 0,
    touch_points: navigator.maxTouchPoints || 0,
    hardware_concurrency: navigator.hardwareConcurrency || 0,
    device_memory_gb: navigator.deviceMemory || 0,
    cookies_enabled: navigator.cookieEnabled,
    platform: navigator.platform || '',
  };

  const update = () => {
    try {
      field.value = JSON.stringify(details);
    } catch (_) {
      field.value = '';
    }
  };
  update();

  const uaData = navigator.userAgentData;
  if (!uaData) return;

  details.brands = (uaData.brands || []).map((brand) => `${brand.brand} ${brand.version}`);
  details.mobile = uaData.mobile;
  details.platform = uaData.platform || details.platform;
  update();

  if (typeof uaData.getHighEntropyValues !== 'function') return;
  uaData.getHighEntropyValues([
    'architecture', 'bitness', 'model', 'platformVersion', 'fullVersionList',
  ]).then((highEntropy) => {
    details.architecture = highEntropy.architecture || '';
    details.bitness = highEntropy.bitness || '';
    details.model = highEntropy.model || '';
    details.platform_version = highEntropy.platformVersion || '';
    if (highEntropy.fullVersionList) {
      details.brands = highEntropy.fullVersionList.map((brand) => `${brand.brand} ${brand.version}`);
    }
    update();
  }).catch(() => {});
})();
