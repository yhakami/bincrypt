// BinCrypt Client-Side Encryption/Decryption
// Uses Web Crypto API for AES-256-GCM encryption

// Derive key from password using PBKDF2
async function deriveKey(password, salt) {
    const encoder = new TextEncoder();
    const passwordBuffer = encoder.encode(password);
    
    const importedKey = await crypto.subtle.importKey(
        'raw',
        passwordBuffer,
        'PBKDF2',
        false,
        ['deriveKey']
    );
    
    return crypto.subtle.deriveKey(
        {
            name: 'PBKDF2',
            salt: salt,
            iterations: 100000,
            hash: 'SHA-256'
        },
        importedKey,
        {
            name: 'AES-GCM',
            length: 256
        },
        false,
        ['encrypt', 'decrypt']
    );
}

// Encrypt paste content
async function encryptPaste(content, password) {
    const encoder = new TextEncoder();
    const data = encoder.encode(content);
    
    // Generate random salt and IV
    const salt = crypto.getRandomValues(new Uint8Array(16));
    const iv = crypto.getRandomValues(new Uint8Array(12));
    
    // Derive key from password
    const key = await deriveKey(password, salt);
    
    // Encrypt data
    const encryptedData = await crypto.subtle.encrypt(
        {
            name: 'AES-GCM',
            iv: iv
        },
        key,
        data
    );
    
    // Combine salt, iv, and encrypted data
    const combined = new Uint8Array(salt.length + iv.length + encryptedData.byteLength);
    combined.set(salt, 0);
    combined.set(iv, salt.length);
    combined.set(new Uint8Array(encryptedData), salt.length + iv.length);
    
    // Return base64 encoded
    return btoa(String.fromCharCode(...combined));
}

// Decrypt paste content
async function decryptPaste(ciphertext, password) {
    try {
        // Decode from base64
        const combined = Uint8Array.from(atob(ciphertext), c => c.charCodeAt(0));
        
        // Extract salt, iv, and encrypted data
        const salt = combined.slice(0, 16);
        const iv = combined.slice(16, 28);
        const encryptedData = combined.slice(28);
        
        // Derive key from password
        const key = await deriveKey(password, salt);
        
        // Decrypt data
        const decryptedData = await crypto.subtle.decrypt(
            {
                name: 'AES-GCM',
                iv: iv
            },
            key,
            encryptedData
        );
        
        // Decode and return text
        const decoder = new TextDecoder();
        return decoder.decode(decryptedData);
    } catch (error) {
        console.error('Decryption error:', error);
        throw new Error('Failed to decrypt. Please check your password.');
    }
}

// Utility function to format byte sizes
function formatBytes(bytes) {
    if (bytes === 0) return '0 Bytes';
    
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// Check if browser supports required crypto features
function checkCryptoSupport() {
    if (!window.crypto || !window.crypto.subtle) {
        alert('Your browser does not support the Web Crypto API. Please use a modern browser.');
        return false;
    }
    return true;
}

// Initialize crypto check on page load
document.addEventListener('DOMContentLoaded', function() {
    checkCryptoSupport();
});

// Make functions available globally
window.encryptPaste = encryptPaste;
window.decryptPaste = decryptPaste;
window.deriveKey = deriveKey;
window.formatBytes = formatBytes;
window.checkCryptoSupport = checkCryptoSupport;