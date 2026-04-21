// BinCrypt Client-Side Encryption/Decryption
// Uses Web Crypto API for AES-256-GCM encryption

'use strict';

// Fixed iteration count for cross-device consistency (OWASP 2023 recommendation)
// Using a fixed count ensures pastes encrypted on any device can be decrypted on any other device.
const PBKDF2_ITERATIONS = 600000;

// Crypto constants - frozen to prevent tampering
const CRYPTO_CONSTANTS = Object.freeze({
    SALT_LENGTH: 16,
    IV_LENGTH: 12,
    HEADER_LENGTH: 28  // SALT_LENGTH + IV_LENGTH
});

// Chunked Base64 encoding to avoid call stack overflow on large arrays
// Using spread operator (...) with large arrays causes "Maximum call stack size exceeded"
function bytesToBase64(bytes) {
    const CHUNK_SIZE = 0x8000; // 32KB chunks - safe for all JS engines
    let binary = '';
    for (let i = 0; i < bytes.length; i += CHUNK_SIZE) {
        const chunk = bytes.subarray(i, Math.min(i + CHUNK_SIZE, bytes.length));
        binary += String.fromCharCode.apply(null, chunk);
    }
    return btoa(binary);
}

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
            iterations: PBKDF2_ITERATIONS,
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
    // Input validation
    if (typeof content !== 'string') {
        throw new Error('Content must be a string');
    }
    if (typeof password !== 'string' || password.length === 0) {
        throw new Error('Password must be a non-empty string');
    }

    const encoder = new TextEncoder();
    const data = encoder.encode(content);

    // Generate random salt and IV
    const salt = crypto.getRandomValues(new Uint8Array(CRYPTO_CONSTANTS.SALT_LENGTH));
    const iv = crypto.getRandomValues(new Uint8Array(CRYPTO_CONSTANTS.IV_LENGTH));
    
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

    // Return base64 encoded using chunked encoding to avoid stack overflow on large files
    return bytesToBase64(combined);
}

// Decrypt paste content
async function decryptPaste(ciphertext, password) {
    // Input validation
    if (typeof ciphertext !== 'string' || ciphertext.length === 0) {
        throw new Error('Ciphertext must be a non-empty string');
    }
    if (typeof password !== 'string' || password.length === 0) {
        throw new Error('Password must be a non-empty string');
    }

    try {
        // Decode from base64 - wrap in try to catch malformed input
        let combined;
        try {
            combined = Uint8Array.from(atob(ciphertext), c => c.charCodeAt(0));
        } catch (e) {
            throw new Error('Invalid ciphertext format');
        }

        // Validate minimum length (salt + IV + at least 1 byte + auth tag)
        if (combined.length < CRYPTO_CONSTANTS.HEADER_LENGTH + 17) {
            throw new Error('Ciphertext too short');
        }

        // Extract salt, iv, and encrypted data
        const salt = combined.slice(0, CRYPTO_CONSTANTS.SALT_LENGTH);
        const iv = combined.slice(CRYPTO_CONSTANTS.SALT_LENGTH, CRYPTO_CONSTANTS.HEADER_LENGTH);
        const encryptedData = combined.slice(CRYPTO_CONSTANTS.HEADER_LENGTH);
        
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
        // SECURITY: Don't log error details that could help attackers
        console.error('Decryption failed');
        throw new Error('Failed to decrypt. Please check your password.');
    }
}

// Utility function to format byte sizes
function formatBytes(bytes) {
    // Guard against non-numeric or negative values
    if (typeof bytes !== 'number' || !Number.isFinite(bytes) || bytes < 0) {
        return '0 Bytes';
    }
    if (bytes === 0) return '0 Bytes';

    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB'];
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1);

    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// Check if browser supports required crypto features
// Sets window.cryptoSupported for components to check
function checkCryptoSupport() {
    const supported = !!(window.crypto && window.crypto.subtle);
    window.cryptoSupported = supported;

    if (!supported) {
        // Store error message for UI to display
        window.cryptoError = 'Your browser does not support the Web Crypto API. Please use a modern browser (Chrome 37+, Firefox 34+, Safari 11+, Edge 12+).';
    }

    return supported;
}

// Initialize crypto check on page load
document.addEventListener('DOMContentLoaded', function() {
    if (!checkCryptoSupport()) {
        // Disable all paste functionality by setting global flag
        document.body.classList.add('crypto-unsupported');
    }
});

// Make functions available globally
window.encryptPaste = encryptPaste;
window.decryptPaste = decryptPaste;
window.deriveKey = deriveKey;
window.formatBytes = formatBytes;
window.checkCryptoSupport = checkCryptoSupport;
window.getIterationCount = () => PBKDF2_ITERATIONS;