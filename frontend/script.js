const apiUrl = '/api/items'; // Use relative path for Nginx proxy
const itemList = document.getElementById('item-list');
const addItemForm = document.getElementById('add-item-form');
const itemInput = document.getElementById('item-input');
const quantityInput = document.getElementById('quantity-input');

// --- Functions ---

// Fetch items from backend and render list
const fetchItems = async () => {
    try {
        const response = await fetch(apiUrl);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const items = await response.json();
        renderList(items || []); // Handle null response from backend
    } catch (error) {
        console.error('Error fetching items:', error);
        itemList.innerHTML = '<li>Error loading items. Please try again later.</li>';
    }
};

// Render the list of items in the UL
const renderList = (items) => {
    itemList.innerHTML = ''; // Clear current list
    if (items.length === 0) {
        itemList.innerHTML = '<li>Your shopping list is empty!</li>';
        return;
    }
    items.forEach(item => {
        const li = document.createElement('li');
        li.innerHTML = `
            <span><strong>${escapeHtml(item.name)}</strong> - ${escapeHtml(item.quantity)}</span>
            <button class="delete-btn" data-id="${item.id}">Delete</button>
        `;
        // Add event listener to the delete button
        li.querySelector('.delete-btn').addEventListener('click', handleDeleteItem);
        itemList.appendChild(li);
    });
};

// Handle form submission to add a new item
const handleAddItem = async (event) => {
    event.preventDefault(); // Prevent default form submission

    const name = itemInput.value.trim();
    const quantity = quantityInput.value.trim();

    if (!name || !quantity) {
        alert('Please enter both item name and quantity.');
        return;
    }

    try {
        const response = await fetch(apiUrl, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ name, quantity }),
        });

        if (!response.ok) {
             const errorData = await response.text(); // Read error response if any
             throw new Error(`HTTP error! status: ${response.status} - ${errorData}`);
        }

        // Clear the form
        itemInput.value = '';
        quantityInput.value = '';

        // Refresh the list
        fetchItems();

    } catch (error) {
        console.error('Error adding item:', error);
        alert('Failed to add item. Please check the console for details.');
    }
};

// Handle clicking the delete button
const handleDeleteItem = async (event) => {
    const itemId = event.target.dataset.id;
    if (!itemId) return;

    if (!confirm('Are you sure you want to delete this item?')) {
      return;
    }

    try {
        const response = await fetch(`${apiUrl}/${itemId}`, {
            method: 'DELETE',
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        // Refresh the list
        fetchItems();

    } catch (error) {
        console.error('Error deleting item:', error);
        alert('Failed to delete item.');
    }
};

// Simple HTML escaping function to prevent basic XSS in the display
function escapeHtml(unsafe) {
    if (unsafe === null || unsafe === undefined) return '';
    return unsafe
         .toString()
         .replace(/&/g, "&")
         .replace(/</g, "<")
         .replace(/>/g, ">")
         .replace(/"/g, "&quot;")
         .replace(/'/g, "'");
 }

// --- Initial Load ---
fetchItems(); // Load items when the page loads

// --- Event Listeners ---
addItemForm.addEventListener('submit', handleAddItem);