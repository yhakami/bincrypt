// BinCrypt Paste Viewer
// Note: The main viewer logic is now embedded in viewer.html for better integration
// This file is kept for backward compatibility and potential future enhancements

// Utility function to detect programming language from content
function detectLanguage(content) {
    // Simple language detection based on common patterns
    const lines = content.split('\n').slice(0, 10); // Check first 10 lines
    const text = lines.join('\n').toLowerCase();
    
    // Common file extensions or patterns
    if (text.includes('<!doctype') || text.includes('<html')) return 'html';
    if (text.includes('function') && text.includes('{') && text.includes('}')) {
        if (text.includes('const ') || text.includes('let ') || text.includes('var ')) return 'javascript';
        if (text.includes('func ') && text.includes('package ')) return 'go';
    }
    if (text.includes('def ') && text.includes(':')) return 'python';
    if (text.includes('<?php')) return 'php';
    if (text.includes('#include') || text.includes('int main')) return 'cpp';
    if (text.includes('public class') || text.includes('import java.')) return 'java';
    if (text.includes('fn ') && text.includes('->')) return 'rust';
    if (text.includes('SELECT') || text.includes('INSERT') || text.includes('UPDATE')) return 'sql';
    if (text.includes('```') || text.includes('# ')) return 'markdown';
    if (text.includes('{') && (text.includes('"') || text.includes("'"))) return 'json';
    
    return 'plaintext';
}

// Language detection for backward compatibility
function detectLanguageFromContent(content) {
    return detectLanguage(content);
}

// Make functions available globally for backward compatibility
window.detectLanguage = detectLanguage;
window.detectLanguageFromContent = detectLanguageFromContent;