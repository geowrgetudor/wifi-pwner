// Status update functionality
function updateStatus() {
    fetch('/api/status')
        .then(response => response.json())
        .then(data => {
            const scanningStatus = document.getElementById('scanning-status');
            const crackingStatus = document.getElementById('cracking-status');
            const scanningIcon = document.getElementById('scanning-icon');
            const crackingIcon = document.getElementById('cracking-icon');
            
            if (data.scanning) {
                scanningStatus.textContent = 'Active';
                scanningStatus.className = 'text-sm font-medium text-green-600';
                scanningIcon.style.color = '#28a745';
            } else {
                scanningStatus.textContent = 'Inactive';
                scanningStatus.className = 'text-sm font-medium text-gray-500';
                scanningIcon.style.color = '#6c757d';
            }
            
            if (data.cracking) {
                crackingStatus.textContent = 'Active';
                crackingStatus.className = 'text-sm font-medium text-yellow-600';
                crackingIcon.style.color = '#ffc107';
            } else {
                crackingStatus.textContent = 'Inactive';
                crackingStatus.className = 'text-sm font-medium text-gray-500';
                crackingIcon.style.color = '#6c757d';
            }
            
            // Update statistics if available
            if (data.totalAPs !== undefined) {
                document.getElementById('total-aps').textContent = data.totalAPs;
            }
            if (data.totalProbes !== undefined) {
                document.getElementById('total-probes').textContent = data.totalProbes;
            }
        })
        .catch(error => {
            console.error('Error updating status:', error);
        });
}

// Control button functionality
document.getElementById('toggle-scanning').addEventListener('click', function() {
    fetch('/api/toggle-scanning', { method: 'POST' })
        .then(response => response.json())
        .then(data => {
            updateStatus();
        })
        .catch(error => {
            console.error('Error toggling scanning:', error);
        });
});

document.getElementById('toggle-cracking').addEventListener('click', function() {
    fetch('/api/toggle-cracking', { method: 'POST' })
        .then(response => response.json())
        .then(data => {
            updateStatus();
        })
        .catch(error => {
            console.error('Error toggling cracking:', error);
        });
});

// Update status every 5 seconds
updateStatus();
setInterval(updateStatus, 5000);