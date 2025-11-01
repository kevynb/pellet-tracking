(function () {
  function formatEuro(value) {
    const formatter = new Intl.NumberFormat('fr-FR', {
      style: 'currency',
      currency: 'EUR',
    });
    return formatter.format(value);
  }

  function updatePurchaseTotal(form) {
    const bags = parseFloat(form.querySelector('[name="bags"]').value || '0');
    const bagWeight = parseFloat(form.querySelector('[name="bag_weight_kg"]').value || '0');
    const unitPrice = parseFloat(form.querySelector('[name="unit_price_eur"]').value || '0');
    const totalNode = form.querySelector('[data-role="purchase-total"]');
    const weightNode = form.querySelector('[data-role="purchase-weight-total"]');
    if (!totalNode) return;
    const total = bags * unitPrice;
    totalNode.textContent = formatEuro(isFinite(total) ? total : 0);
    if (weightNode) {
      const totalWeight = bags * bagWeight;
      const formatted = isFinite(totalWeight) ? totalWeight.toFixed(1).replace('.', ',') : '0,0';
      weightNode.textContent = formatted;
    }
  }

  document.addEventListener('input', function (event) {
    const form = event.target.closest('[data-controller="purchase-form"]');
    if (!form) return;
    if (event.target.matches('[name="bags"], [name="unit_price_eur"], [name="bag_weight_kg"]')) {
      updatePurchaseTotal(form);
    }
  });

  document.addEventListener('DOMContentLoaded', function () {
    document.querySelectorAll('input[type="date"][data-default-today="true"]').forEach(function (input) {
      if (!input.value) {
        const today = new Date();
        const month = String(today.getMonth() + 1).padStart(2, '0');
        const day = String(today.getDate()).padStart(2, '0');
        input.value = `${today.getFullYear()}-${month}-${day}`;
      }
    });

    document.querySelectorAll('[data-controller="purchase-form"]').forEach(function (form) {
      updatePurchaseTotal(form);
    });
  });
})();
