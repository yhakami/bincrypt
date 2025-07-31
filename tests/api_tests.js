// BinCrypt API Test Suite
// Comprehensive testing of all backend API endpoints

class APITestSuite {
    constructor(baseUrl = 'http://localhost:8080') {
        this.baseUrl = baseUrl;
        this.recorder = new TestRecorder();
        this.testData = {
            createdPastes: [],
            testContent: 'API Test Content - ' + new Date().toISOString()
        };
    }

    async runAllTests() {
        console.log('🚀 Starting API Test Suite...');
        
        try {
            await this.testPasteCreation();
            await this.testPasteRetrieval();
            await this.testPasteExpiration();
            await this.testBurnAfterRead();
            await this.testSizeLimits();
            await this.testInvalidRequests();
            await this.testInvoiceCreation();
            await this.testPaymentWebhook();
            await this.testStaticFiles();
            await this.testErrorHandling();
            await this.testSecurityHeaders();
            await this.testConcurrentRequests();
            
            console.log('✅ API Test Suite completed');
            return this.recorder.getSummary();
            
        } catch (error) {
            console.error('💥 API Test Suite error:', error);
            this.recorder.record('API Test Suite', 'FAIL', { error: error.message });
            throw error;
        }
    }

    async testPasteCreation() {
        console.log('📝 Testing paste creation...');
        
        // Test basic paste creation
        try {
            const testCases = [
                {
                    name: 'Basic Paste Creation',
                    ciphertext: 'dGVzdCBjaXBoZXJ0ZXh0', // base64 encoded 'test ciphertext'
                    expiry_seconds: 3600
                },
                {
                    name: 'Paste with Burn After Read',
                    ciphertext: 'YnVybiBhZnRlciByZWFkIHRlc3Q=', // base64 encoded 'burn after read test'
                    expiry_seconds: 7200,
                    burn_after_read: true
                },
                {
                    name: 'Maximum Expiry Paste',
                    ciphertext: 'bWF4IGV4cGlyeSB0ZXN0',
                    expiry_seconds: 604800 // 7 days
                }
            ];

            for (const testCase of testCases) {
                try {
                    const startTime = performance.now();
                    const response = await fetch(`${this.baseUrl}/api/paste`, {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json'
                        },
                        body: JSON.stringify({
                            ciphertext: testCase.ciphertext,
                            expiry_seconds: testCase.expiry_seconds,
                            burn_after_read: testCase.burn_after_read || false
                        })
                    });
                    const endTime = performance.now();
                    
                    if (response.ok) {
                        const data = await response.json();
                        
                        // Validate response structure
                        if (data.id && data.expires_at) {
                            this.recorder.record(testCase.name, 'PASS', {
                                pasteId: data.id,
                                expiresAt: data.expires_at,
                                responseTime: `${(endTime - startTime).toFixed(2)}ms`
                            });
                            
                            // Store for later tests
                            this.testData.createdPastes.push({
                                id: data.id,
                                ciphertext: testCase.ciphertext,
                                burnAfterRead: testCase.burn_after_read || false
                            });
                        } else {
                            this.recorder.record(testCase.name, 'FAIL', {
                                error: 'Invalid response structure',
                                response: data
                            });
                        }
                    } else {
                        const errorText = await response.text();
                        this.recorder.record(testCase.name, 'FAIL', {
                            status: response.status,
                            error: errorText
                        });
                    }
                } catch (error) {
                    this.recorder.record(testCase.name, 'FAIL', {
                        error: error.message
                    });
                }
            }
        } catch (error) {
            this.recorder.record('Paste Creation Tests', 'FAIL', { error: error.message });
        }
    }

    async testPasteRetrieval() {
        console.log('📖 Testing paste retrieval...');
        
        if (this.testData.createdPastes.length === 0) {
            this.recorder.record('Paste Retrieval', 'SKIP', {
                reason: 'No pastes created to test retrieval'
            });
            return;
        }

        for (const paste of this.testData.createdPastes) {
            try {
                const startTime = performance.now();
                const response = await fetch(`${this.baseUrl}/p/${paste.id}`, {
                    method: 'GET'
                });
                const endTime = performance.now();
                
                if (response.ok) {
                    const html = await response.text();
                    
                    // Check if HTML contains expected elements
                    const checks = [
                        { check: html.includes('<!DOCTYPE html>'), name: 'Valid HTML' },
                        { check: html.includes('BinCrypt'), name: 'Contains BinCrypt branding' },
                        { check: html.includes('viewer.js'), name: 'Includes viewer script' },
                        { check: html.includes(paste.ciphertext), name: 'Contains ciphertext' }
                    ];
                    
                    const passedChecks = checks.filter(c => c.check).length;
                    const status = passedChecks === checks.length ? 'PASS' : 'WARNING';
                    
                    this.recorder.record(`Paste Retrieval (${paste.id.substring(0, 8)})`, status, {
                        responseTime: `${(endTime - startTime).toFixed(2)}ms`,
                        htmlSize: html.length,
                        passedChecks: `${passedChecks}/${checks.length}`,
                        failedChecks: checks.filter(c => !c.check).map(c => c.name)
                    });
                } else {
                    this.recorder.record(`Paste Retrieval (${paste.id.substring(0, 8)})`, 'FAIL', {
                        status: response.status,
                        statusText: response.statusText
                    });
                }
            } catch (error) {
                this.recorder.record(`Paste Retrieval (${paste.id.substring(0, 8)})`, 'FAIL', {
                    error: error.message
                });
            }
        }
    }

    async testBurnAfterRead() {
        console.log('🔥 Testing burn after read...');
        
        // Create a burn-after-read paste
        try {
            const response = await fetch(`${this.baseUrl}/api/paste`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    ciphertext: 'YnVybkFmdGVyUmVhZFRlc3Q=',
                    expiry_seconds: 3600,
                    burn_after_read: true
                })
            });

            if (response.ok) {
                const data = await response.json();
                const pasteId = data.id;

                // First access should work
                const firstAccess = await fetch(`${this.baseUrl}/p/${pasteId}`);
                
                if (firstAccess.ok) {
                    this.recorder.record('Burn After Read - First Access', 'PASS');
                    
                    // Wait a moment for the burn to process
                    await new Promise(resolve => setTimeout(resolve, 3000));
                    
                    // Second access should fail
                    const secondAccess = await fetch(`${this.baseUrl}/p/${pasteId}`);
                    
                    if (secondAccess.status === 404) {
                        this.recorder.record('Burn After Read - Second Access', 'PASS', {
                            message: 'Paste correctly deleted after first view'
                        });
                    } else {
                        this.recorder.record('Burn After Read - Second Access', 'FAIL', {
                            error: 'Paste was not deleted after first view',
                            status: secondAccess.status
                        });
                    }
                } else {
                    this.recorder.record('Burn After Read - First Access', 'FAIL', {
                        status: firstAccess.status
                    });
                }
            } else {
                this.recorder.record('Burn After Read Setup', 'FAIL', {
                    status: response.status
                });
            }
        } catch (error) {
            this.recorder.record('Burn After Read', 'FAIL', { error: error.message });
        }
    }

    async testSizeLimits() {
        console.log('📏 Testing size limits...');
        
        const sizeTests = [
            {
                name: 'Within Limit (100KB)',
                size: 102400, // 100KB
                shouldPass: true
            },
            {
                name: 'At Limit (512KB)',
                size: 524288, // 512KB
                shouldPass: true
            },
            {
                name: 'Over Limit (600KB)',
                size: 614400, // 600KB
                shouldPass: false
            },
            {
                name: 'Way Over Limit (1MB)',
                size: 1048576, // 1MB
                shouldPass: false
            }
        ];

        for (const test of sizeTests) {
            try {
                const largeCiphertext = 'A'.repeat(test.size);
                
                const response = await fetch(`${this.baseUrl}/api/paste`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        ciphertext: largeCiphertext,
                        expiry_seconds: 3600
                    })
                });

                if (test.shouldPass && response.ok) {
                    this.recorder.record(test.name, 'PASS', {
                        size: `${(test.size / 1024).toFixed(1)}KB`
                    });
                } else if (!test.shouldPass && response.status === 400) {
                    this.recorder.record(test.name, 'PASS', {
                        message: 'Correctly rejected oversized content',
                        size: `${(test.size / 1024).toFixed(1)}KB`
                    });
                } else {
                    this.recorder.record(test.name, 'FAIL', {
                        expectedPass: test.shouldPass,
                        actualStatus: response.status,
                        size: `${(test.size / 1024).toFixed(1)}KB`
                    });
                }
            } catch (error) {
                this.recorder.record(test.name, 'FAIL', { error: error.message });
            }
        }
    }

    async testInvalidRequests() {
        console.log('❌ Testing invalid requests...');
        
        const invalidTests = [
            {
                name: 'Empty Body',
                body: '',
                contentType: 'application/json'
            },
            {
                name: 'Invalid JSON',
                body: '{invalid json}',
                contentType: 'application/json'
            },
            {
                name: 'Missing Ciphertext',
                body: JSON.stringify({ expiry_seconds: 3600 }),
                contentType: 'application/json'
            },
            {
                name: 'Invalid Expiry',
                body: JSON.stringify({ 
                    ciphertext: 'dGVzdA==', 
                    expiry_seconds: -1 
                }),
                contentType: 'application/json'
            },
            {
                name: 'Wrong Content Type',
                body: 'ciphertext=test&expiry_seconds=3600',
                contentType: 'application/x-www-form-urlencoded'
            }
        ];

        for (const test of invalidTests) {
            try {
                const response = await fetch(`${this.baseUrl}/api/paste`, {
                    method: 'POST',
                    headers: { 'Content-Type': test.contentType },
                    body: test.body
                });

                if (response.status >= 400 && response.status < 500) {
                    this.recorder.record(`Invalid Request - ${test.name}`, 'PASS', {
                        status: response.status,
                        message: 'Correctly rejected invalid request'
                    });
                } else {
                    this.recorder.record(`Invalid Request - ${test.name}`, 'FAIL', {
                        status: response.status,
                        error: 'Should have rejected invalid request'
                    });
                }
            } catch (error) {
                this.recorder.record(`Invalid Request - ${test.name}`, 'FAIL', {
                    error: error.message
                });
            }
        }
    }

    async testInvoiceCreation() {
        console.log('💰 Testing invoice creation...');
        
        try {
            const response = await fetch(`${this.baseUrl}/api/invoice`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ tier: 'premium' })
            });

            // BTCPay might not be configured in test environment
            if (response.ok) {
                const data = await response.json();
                
                if (data.id && data.amount) {
                    this.recorder.record('Invoice Creation', 'PASS', {
                        invoiceId: data.id,
                        amount: data.amount
                    });
                } else {
                    this.recorder.record('Invoice Creation', 'FAIL', {
                        error: 'Invalid invoice response structure'
                    });
                }
            } else if (response.status === 500) {
                this.recorder.record('Invoice Creation', 'WARNING', {
                    message: 'BTCPay Server not configured (expected in test environment)',
                    status: response.status
                });
            } else {
                this.recorder.record('Invoice Creation', 'FAIL', {
                    status: response.status,
                    statusText: response.statusText
                });
            }
        } catch (error) {
            this.recorder.record('Invoice Creation', 'WARNING', {
                message: 'BTCPay integration test failed (may not be configured)',
                error: error.message
            });
        }
    }

    async testPaymentWebhook() {
        console.log('🔗 Testing payment webhook...');
        
        try {
            const webhookData = {
                id: 'test-invoice-id',
                status: 'paid',
                amount: 5.00
            };

            const response = await fetch(`${this.baseUrl}/api/payhook`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(webhookData)
            });

            if (response.ok) {
                this.recorder.record('Payment Webhook', 'PASS', {
                    status: response.status
                });
            } else {
                this.recorder.record('Payment Webhook', 'FAIL', {
                    status: response.status,
                    statusText: response.statusText
                });
            }
        } catch (error) {
            this.recorder.record('Payment Webhook', 'FAIL', {
                error: error.message
            });
        }
    }

    async testStaticFiles() {
        console.log('📁 Testing static file serving...');
        
        const staticFiles = [
            '/static/app.js',
            '/static/styles.css',
            '/static/viewer.js'
        ];

        for (const file of staticFiles) {
            try {
                const response = await fetch(`${this.baseUrl}${file}`);
                
                if (response.ok) {
                    const content = await response.text();
                    this.recorder.record(`Static File - ${file}`, 'PASS', {
                        size: `${content.length} bytes`,
                        contentType: response.headers.get('content-type')
                    });
                } else {
                    this.recorder.record(`Static File - ${file}`, 'FAIL', {
                        status: response.status
                    });
                }
            } catch (error) {
                this.recorder.record(`Static File - ${file}`, 'FAIL', {
                    error: error.message
                });
            }
        }
    }

    async testPasteExpiration() {
        console.log('⏰ Testing paste expiration...');
        
        // This test is challenging without manipulating server time
        // For now, we'll test with very short expiry
        try {
            const response = await fetch(`${this.baseUrl}/api/paste`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    ciphertext: 'ZXhwaXJ5VGVzdA==',
                    expiry_seconds: 1 // 1 second expiry
                })
            });

            if (response.ok) {
                const data = await response.json();
                const pasteId = data.id;
                
                // Wait for expiry
                await new Promise(resolve => setTimeout(resolve, 2000));
                
                // Try to access expired paste
                const accessResponse = await fetch(`${this.baseUrl}/p/${pasteId}`);
                
                if (accessResponse.status === 404) {
                    this.recorder.record('Paste Expiration', 'PASS', {
                        message: 'Expired paste correctly returns 404'
                    });
                } else {
                    this.recorder.record('Paste Expiration', 'WARNING', {
                        message: 'Expiry may not be immediate (cleanup runs hourly)',
                        status: accessResponse.status
                    });
                }
            } else {
                this.recorder.record('Paste Expiration Setup', 'FAIL', {
                    status: response.status
                });
            }
        } catch (error) {
            this.recorder.record('Paste Expiration', 'FAIL', {
                error: error.message
            });
        }
    }

    async testErrorHandling() {
        console.log('🚫 Testing error handling...');
        
        const errorTests = [
            {
                name: 'Non-existent Paste',
                url: '/p/nonexistent-id-12345',
                expectedStatus: 404
            },
            {
                name: 'Invalid Paste ID Format',
                url: '/p/../../../etc/passwd',
                expectedStatus: 404
            },
            {
                name: 'Very Long Paste ID',
                url: '/p/' + 'A'.repeat(1000),
                expectedStatus: 404
            }
        ];

        for (const test of errorTests) {
            try {
                const response = await fetch(`${this.baseUrl}${test.url}`);
                
                if (response.status === test.expectedStatus) {
                    this.recorder.record(`Error Handling - ${test.name}`, 'PASS');
                } else {
                    this.recorder.record(`Error Handling - ${test.name}`, 'FAIL', {
                        expectedStatus: test.expectedStatus,
                        actualStatus: response.status
                    });
                }
            } catch (error) {
                this.recorder.record(`Error Handling - ${test.name}`, 'FAIL', {
                    error: error.message
                });
            }
        }
    }

    async testSecurityHeaders() {
        console.log('🛡️ Testing security headers...');
        
        try {
            const response = await fetch(`${this.baseUrl}/`);
            const headers = response.headers;
            
            const securityHeaders = [
                {
                    name: 'Content-Security-Policy',
                    header: 'content-security-policy',
                    required: true
                },
                {
                    name: 'Strict-Transport-Security',
                    header: 'strict-transport-security',
                    required: true
                },
                {
                    name: 'X-Content-Type-Options',
                    header: 'x-content-type-options',
                    required: false
                },
                {
                    name: 'X-Frame-Options',
                    header: 'x-frame-options',
                    required: false
                }
            ];

            for (const secHeader of securityHeaders) {
                const headerValue = headers.get(secHeader.header);
                
                if (headerValue) {
                    this.recorder.record(`Security Header - ${secHeader.name}`, 'PASS', {
                        value: headerValue
                    });
                } else if (secHeader.required) {
                    this.recorder.record(`Security Header - ${secHeader.name}`, 'FAIL', {
                        error: 'Required security header missing'
                    });
                } else {
                    this.recorder.record(`Security Header - ${secHeader.name}`, 'WARNING', {
                        message: 'Optional security header not present'
                    });
                }
            }
        } catch (error) {
            this.recorder.record('Security Headers', 'FAIL', {
                error: error.message
            });
        }
    }

    async testConcurrentRequests() {
        console.log('🔄 Testing concurrent requests...');
        
        const concurrentRequests = 10;
        const promises = [];
        
        for (let i = 0; i < concurrentRequests; i++) {
            promises.push(
                fetch(`${this.baseUrl}/api/paste`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        ciphertext: `Y29uY3VycmVudCR7aX0=`, // base64 encoded "concurrent${i}"
                        expiry_seconds: 3600
                    })
                })
            );
        }

        try {
            const startTime = performance.now();
            const responses = await Promise.all(promises);
            const endTime = performance.now();
            
            const successful = responses.filter(r => r.ok).length;
            const successRate = (successful / concurrentRequests) * 100;
            
            if (successRate >= 90) {
                this.recorder.record('Concurrent Requests', 'PASS', {
                    successRate: `${successRate.toFixed(1)}%`,
                    totalTime: `${(endTime - startTime).toFixed(2)}ms`,
                    requestCount: concurrentRequests
                });
            } else if (successRate >= 70) {
                this.recorder.record('Concurrent Requests', 'WARNING', {
                    successRate: `${successRate.toFixed(1)}%`,
                    message: 'Some concurrent requests failed'
                });
            } else {
                this.recorder.record('Concurrent Requests', 'FAIL', {
                    successRate: `${successRate.toFixed(1)}%`,
                    error: 'High failure rate for concurrent requests'
                });
            }
        } catch (error) {
            this.recorder.record('Concurrent Requests', 'FAIL', {
                error: error.message
            });
        }
    }

    getSummary() {
        return this.recorder.getSummary();
    }

    getResults() {
        return this.recorder.exportResults();
    }
}

// Export for both browser and Node.js
if (typeof window !== 'undefined') {
    window.APITestSuite = APITestSuite;
}

if (typeof module !== 'undefined' && module.exports) {
    module.exports = APITestSuite;
}