let cart;

function checkout() {
  return cart.items.length;
}

addEventListener("DOMContentLoaded", () => {
  console.log("shop ready", { items: 0 });
  document.getElementById("checkout").addEventListener("click", checkout);
});
