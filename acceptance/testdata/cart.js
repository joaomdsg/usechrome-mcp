async function addToCart() {
  const res = await fetch("api/cart.json", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ sku: "tea", qty: 1 }),
  });
  const cart = await res.json();
  document.getElementById("count").textContent = cart.items;
}

addEventListener("DOMContentLoaded", () => {
  document.getElementById("add").addEventListener("click", addToCart);
});
