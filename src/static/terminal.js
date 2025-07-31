// BinCrypt Terminal UI JavaScript

function binCrypt() {
    return {
        // View state
        view: 'paste',
        loading: false,
        error: '',
        shareUrl: '',
        
        // Form data
        content: '',
        encryptionPassword: '',
        settings: {
            syntaxHighlight: false,
            language: 'auto',
            expirationDate: null,
            burnAfterRead: false
        },
        
        // Stats
        stats: {
            lines: 0,
            size: '0 bytes'
        },
        
        // Password strength
        passwordStrength: {
            percent: 0,
            level: 'weak',
            text: ''
        },
        
        // UI state
        isPremium: false,
        showPremium: false,
        invoice: null,
        invoiceLoading: false,
        recentPastes: [],
        
        // Computed
        expirationHint: '',
        
        init() {
            // Initialize date picker
            const today = new Date();
            const maxDate = new Date();
            maxDate.setDate(today.getDate() + (this.isPremium ? 90 : 12));
            
            const defaultDate = new Date();
            defaultDate.setDate(today.getDate() + 12);
            
            flatpickr(this.$refs.datepicker, {
                theme: "dark",
                minDate: today,
                maxDate: maxDate,
                defaultDate: defaultDate,
                dateFormat: "Y-m-d",
                onChange: (selectedDates) => {
                    this.settings.expirationDate = selectedDates[0];
                    this.updateExpirationHint();
                }
            });
            
            this.settings.expirationDate = defaultDate;
            this.updateExpirationHint();
            
            // Load recent pastes from localStorage
            this.loadRecentPastes();
            
            // Check premium status
            this.checkPremiumStatus();
        },
        
        updateStats() {
            const bytes = new TextEncoder().encode(this.content).length;
            const lines = this.content.split('\n').length;
            
            this.stats.lines = lines;
            
            if (bytes < 1024) {
                this.stats.size = bytes + ' bytes';
            } else if (bytes < 1048576) {
                this.stats.size = (bytes / 1024).toFixed(1) + ' KB';
            } else {
                this.stats.size = (bytes / 1048576).toFixed(2) + ' MB';
            }
        },
        
        updateExpirationHint() {
            if (!this.settings.expirationDate) return;
            
            const now = new Date();
            const exp = new Date(this.settings.expirationDate);
            const days = Math.ceil((exp - now) / (1000 * 60 * 60 * 24));
            
            this.expirationHint = `Expires in ${days} day${days !== 1 ? 's' : ''}`;
        },
        
        checkPasswordStrength() {
            const password = this.encryptionPassword;
            let strength = 0;
            
            // Length
            if (password.length >= 8) strength += 20;
            if (password.length >= 12) strength += 20;
            if (password.length >= 16) strength += 20;
            
            // Character variety
            if (/[a-z]/.test(password)) strength += 10;
            if (/[A-Z]/.test(password)) strength += 10;
            if (/[0-9]/.test(password)) strength += 10;
            if (/[^a-zA-Z0-9]/.test(password)) strength += 10;
            
            this.passwordStrength.percent = strength;
            
            if (strength < 40) {
                this.passwordStrength.level = 'weak';
                this.passwordStrength.text = 'WEAK';
            } else if (strength < 70) {
                this.passwordStrength.level = 'medium';
                this.passwordStrength.text = 'MEDIUM';
            } else {
                this.passwordStrength.level = 'strong';
                this.passwordStrength.text = 'STRONG';
            }
        },
        
        async createPaste() {
            this.error = '';
            this.loading = true;
            
            try {
                // Prepare metadata
                const metadata = {
                    syntaxHighlight: this.settings.syntaxHighlight,
                    language: this.settings.language,
                    burnAfterRead: this.settings.burnAfterRead
                };
                
                // Combine content with metadata
                const fullContent = JSON.stringify({
                    content: this.content,
                    metadata: metadata,
                });
                
                // Encrypt the combined content
                const ciphertext = await window.encryptPaste(fullContent, this.encryptionPassword);
                
                // Calculate expiry seconds
                const now = new Date();
                const exp = new Date(this.settings.expirationDate);
                const expirySeconds = Math.floor((exp - now) / 1000);
                
                // Send to server
                const response = await fetch('/api/paste', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        ciphertext: ciphertext,
                        expiry_seconds: expirySeconds,
                        burn_after_read: this.settings.burnAfterRead
                    })
                });
                
                if (!response.ok) {
                    throw new Error(await response.text());
                }
                
                const data = await response.json();
                this.shareUrl = window.location.origin + '/p/' + data.id;
                
                // Save to recent pastes
                this.saveRecentPaste(data.id, this.stats.size);
                
            } catch (err) {
                this.error = err.message || 'Failed to create paste';
            } finally {
                this.loading = false;
            }
        },
        
        async hashPassword(password) {
            const encoder = new TextEncoder();
            const data = encoder.encode(password);
            const hash = await crypto.subtle.digest('SHA-256', data);
            return btoa(String.fromCharCode(...new Uint8Array(hash)));
        },
        
        async copyUrl() {
            try {
                await navigator.clipboard.writeText(this.shareUrl);
                // Visual feedback
                const button = event.target;
                const originalText = button.textContent;
                button.textContent = '[COPIED!]';
                setTimeout(() => {
                    button.textContent = originalText;
                }, 1000);
            } catch (err) {
                alert('Failed to copy link');
            }
        },
        
        resetForm() {
            this.content = '';
            this.encryptionPassword = '';
            this.settings = {
                syntaxHighlight: false,
                language: 'auto',
                expirationDate: new Date(Date.now() + 12 * 24 * 60 * 60 * 1000),
                burnAfterRead: false
            };
            this.updateStats();
            this.checkPasswordStrength();
        },
        
        async createInvoice() {
            this.invoiceLoading = true;
            
            try {
                const response = await fetch('/api/invoice', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ tier: 'premium' })
                });
                
                if (!response.ok) {
                    throw new Error('Failed to create invoice');
                }
                
                this.invoice = await response.json();
                this.showPremium = false;
            } catch (err) {
                alert('Failed to create payment invoice. Please try again.');
            } finally {
                this.invoiceLoading = false;
            }
        },
        
        loadRecentPastes() {
            const stored = localStorage.getItem('bincrypt_recent');
            if (stored) {
                this.recentPastes = JSON.parse(stored);
            }
        },
        
        saveRecentPaste(id, size) {
            const paste = {
                id: id,
                created: new Date().toISOString(),
                size: size
            };
            
            this.recentPastes.unshift(paste);
            
            // Keep only last 20
            if (this.recentPastes.length > 20) {
                this.recentPastes = this.recentPastes.slice(0, 20);
            }
            
            localStorage.setItem('bincrypt_recent', JSON.stringify(this.recentPastes));
        },
        
        formatDate(isoString) {
            const date = new Date(isoString);
            return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
        },
        
        checkPremiumStatus() {
            // Check localStorage for premium status
            const premiumUntil = localStorage.getItem('bincrypt_premium');
            if (premiumUntil) {
                const expiryDate = new Date(premiumUntil);
                if (expiryDate > new Date()) {
                    this.isPremium = true;
                }
            }
        }
    }
}

// Language detection
function detectLanguage(content) {
    // Simple heuristics for language detection
    const patterns = {
        javascript: /(?:function|const|let|var|=>|class|import|export)\s/,
        python: /(?:def|import|from|class|if __name__|print\()/,
        go: /(?:package|func|import|var|const|type)\s/,
        rust: /(?:fn|let|mut|impl|struct|enum|use)\s/,
        cpp: /(?:#include|int main|std::|cout|cin)/,
        java: /(?:public class|private|protected|import java|System\.out)/,
        html: /(?:<html|<body|<div|<script|<style)/i,
        css: /(?:\.[\w-]+\s*\{|#[\w-]+\s*\{|@media|:root)/,
        sql: /(?:SELECT|FROM|WHERE|INSERT|UPDATE|DELETE)\s/i,
        json: /^\s*\{[\s\S]*\}\s*$/,
        markdown: /(?:^#{1,6}\s|^\*\s|^\d+\.\s|\[.*\]\(.*\))/m,
        bash: /(?:^#!\/bin\/bash|^\s*\$\s|echo|export|if \\\[)/m
    };
    
    for (const [lang, pattern] of Object.entries(patterns)) {
        if (pattern.test(content)) {
            return lang;
        }
    }
    
    return 'plaintext';
}