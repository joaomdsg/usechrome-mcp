// Holds the main thread for 300 ms, as a heavy handler would.
function checkout() {
  const end = performance.now() + 300;
  while (performance.now() < end) {}
  document.getElementById("status").textContent = "Paid";
}

document.getElementById("pay").addEventListener("click", checkout);
