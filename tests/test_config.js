// BinCrypt Test Configuration
// Test configuration and utilities for comprehensive testing

const TEST_CONFIG = {
    BASE_URL: 'http://localhost:8080',
    API_BASE: 'http://localhost:8080/api',
    FIREBASE_EMULATOR: 'http://localhost:4000',
    
    // Test timeouts
    TIMEOUT: {
        API_REQUEST: 5000,
        ENCRYPTION: 2000,
        PAGE_LOAD: 10000
    },
    
    // Test data
    TEST_PASSWORDS: [
        'simple123',
        'Complex!Password#123',
        'very-long-password-with-special-characters-and-numbers-12345!@#$%',
        '🔐🛡️SecureEmoji',
        'a', // Single character
        'A'.repeat(100), // Very long
    ],
    
    TEST_CONTENT: {
        SMALL: 'Hello World!',
        MEDIUM: 'Lorem ipsum dolor sit amet, consectetur adipiscing elit. '.repeat(100),
        LARGE: 'Large test content. '.repeat(10000), // ~200KB
        MAX_SIZE: 'X'.repeat(524288), // 512KB - max free tier size
        
        // Code samples for syntax highlighting
        CODE_SAMPLES: {
            javascript: `
function fibonacci(n) {
    if (n <= 1) return n;
    return fibonacci(n - 1) + fibonacci(n - 2);
}
console.log(fibonacci(10));`,
            python: `
def fibonacci(n):
    if n <= 1:
        return n
    return fibonacci(n - 1) + fibonacci(n - 2)

print(fibonacci(10))`,
            html: `
<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body><h1>Hello World</h1></body>
</html>`,
            json: `
{
    "name": "BinCrypt",
    "version": "1.0.0",
    "description": "Secure encrypted pastebin"
}`
        }
    },
    
    // Security test payloads
    SECURITY_PAYLOADS: {
        XSS: [
            '<script>alert("XSS")</script>',
            'javascript:alert("XSS")',
            '<img src=x onerror=alert("XSS")>',
            '\'); alert("XSS"); //',
            '<svg onload=alert("XSS")>'
        ],
        SQL_INJECTION: [
            "'; DROP TABLE users; --",
            "1' OR '1'='1",
            "admin'--",
            "' UNION SELECT * FROM passwords--"
        ],
        COMMAND_INJECTION: [
            "; ls -la",
            "| cat /etc/passwd",
            "&& rm -rf /",
            "`whoami`"
        ],
        OVERSIZED_DATA: 'X'.repeat(1048576) // 1MB
    }
};

// Test utilities
class TestUtils {
    static async sleep(ms) {
        return new Promise(resolve => setTimeout(resolve, ms));
    }
    
    static generateRandomString(length = 20) {
        const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
        let result = '';
        for (let i = 0; i < length; i++) {
            result += chars.charAt(Math.floor(Math.random() * chars.length));
        }
        return result;
    }
    
    static getCurrentTimestamp() {
        return new Date().toISOString();
    }
    
    static measureExecutionTime(func) {
        const start = performance.now();
        const result = func();
        const end = performance.now();
        return {
            result,
            executionTime: end - start
        };
    }
    
    static async measureAsyncExecutionTime(asyncFunc) {
        const start = performance.now();
        const result = await asyncFunc();
        const end = performance.now();
        return {
            result,
            executionTime: end - start
        };
    }
    
    static validateUrl(url) {
        try {
            new URL(url);
            return true;
        } catch {
            return false;
        }
    }
    
    static sanitizeForHTML(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
}

// Test result recorder
class TestRecorder {
    constructor() {
        this.results = [];
        this.startTime = Date.now();
    }
    
    record(testName, status, details = {}) {
        const result = {
            testName,
            status, // 'PASS', 'FAIL', 'SKIP', 'WARNING'
            timestamp: new Date().toISOString(),
            details,
            duration: details.executionTime || 0
        };
        
        this.results.push(result);
        console.log(`[${status}] ${testName}`, details);
        return result;
    }
    
    getSummary() {
        const summary = {
            total: this.results.length,
            passed: this.results.filter(r => r.status === 'PASS').length,
            failed: this.results.filter(r => r.status === 'FAIL').length,
            warnings: this.results.filter(r => r.status === 'WARNING').length,
            skipped: this.results.filter(r => r.status === 'SKIP').length,
            totalDuration: Date.now() - this.startTime,
            averageDuration: this.results.reduce((sum, r) => sum + r.duration, 0) / this.results.length
        };
        
        summary.successRate = ((summary.passed / summary.total) * 100).toFixed(2) + '%';
        return summary;
    }
    
    getFailedTests() {
        return this.results.filter(r => r.status === 'FAIL');
    }
    
    exportResults() {
        return {
            summary: this.getSummary(),
            results: this.results,
            exportTime: new Date().toISOString()
        };
    }
}

// Make available globally
if (typeof window !== 'undefined') {
    window.TEST_CONFIG = TEST_CONFIG;
    window.TestUtils = TestUtils;
    window.TestRecorder = TestRecorder;
}

// Node.js export
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { TEST_CONFIG, TestUtils, TestRecorder };
}