// BinCrypt Coinzilla Integration
// Handles crypto-focused ad loading and placement

class BinCryptCoinzillaAds {
    constructor() {
        this.zoneIds = {
            banner728x90: '', // Will be set from environment
            sidebar300x600: '',
            rectangle300x250: '',
            mobile320x50: ''
        };
        this.adsLoaded = false;
        this.adBlockDetected = false;
        this.isPremiumUser = false;
        
        // Check premium status
        this.checkPremiumStatus();
        
        // Initialize ads if not premium
        if (!this.isPremiumUser) {
            this.init();
        }
    }
    
    checkPremiumStatus() {
        // Check localStorage for premium status
        const premiumUntil = localStorage.getItem('bincrypt_premium');
        if (premiumUntil) {
            const expiryDate = new Date(premiumUntil);
            if (expiryDate > new Date()) {
                this.isPremiumUser = true;
                document.body.classList.add('premium-user');
            }
        }
    }
    
    async init() {
        // Fetch Coinzilla configuration from server
        try {
            const response = await fetch('/api/config');
            const config = await response.json();
            
            if (config.coinzillaZones) {
                this.zoneIds = config.coinzillaZones;
                
                // Load Coinzilla script
                this.loadCoinzillaScript();
                
                // Setup lazy loading
                this.setupLazyLoading();
                
                // Detect ad blockers
                setTimeout(() => this.detectAdBlock(), 1000);
            } else {
                console.info('Coinzilla not configured');
            }
        } catch (err) {
            console.error('Failed to fetch ad configuration:', err);
        }
    }
    
    loadCoinzillaScript() {
        if (document.getElementById('coinzilla-script')) return;
        
        const script = document.createElement('script');
        script.id = 'coinzilla-script';
        script.async = true;
        script.src = 'https://coinzilla.com/publisher.js';
        script.onload = () => {
            this.adsLoaded = true;
            console.info('Coinzilla script loaded');
        };
        script.onerror = () => {
            this.adBlockDetected = true;
            this.showAdBlockMessage();
        };
        document.head.appendChild(script);
    }
    
    setupLazyLoading() {
        // Use Intersection Observer for lazy loading
        const adContainers = document.querySelectorAll('.coinzilla-ad-container:not(.ad-loaded)');
        
        if ('IntersectionObserver' in window) {
            const adObserver = new IntersectionObserver((entries) => {
                entries.forEach(entry => {
                    if (entry.isIntersecting) {
                        this.loadAd(entry.target);
                        adObserver.unobserve(entry.target);
                    }
                });
            }, {
                rootMargin: '200px 0px' // Load 200px before visible
            });
            
            adContainers.forEach(container => adObserver.observe(container));
        } else {
            // Fallback for older browsers
            adContainers.forEach(container => this.loadAd(container));
        }
    }
    
    loadAd(container) {
        if (container.classList.contains('ad-loaded') || this.isPremiumUser) return;
        
        const adType = container.getAttribute('data-ad-type');
        const zoneId = this.zoneIds[adType];
        
        if (!zoneId) {
            console.warn(`No zone ID configured for ad type: ${adType}`);
            return;
        }
        
        // Create Coinzilla ad div
        const adDiv = document.createElement('div');
        adDiv.className = 'coinzilla-ad';
        adDiv.setAttribute('data-zone', zoneId);
        
        // Add label
        const label = document.createElement('div');
        label.className = 'ad-label';
        label.textContent = 'Advertisement';
        
        container.appendChild(label);
        container.appendChild(adDiv);
        container.classList.add('ad-loaded');
        
        // Animate in
        container.style.opacity = '0';
        container.style.transition = 'opacity 0.3s ease-in';
        
        // Initialize Coinzilla ad
        if (window.coinzilla) {
            try {
                window.coinzilla.init();
                setTimeout(() => {
                    container.style.opacity = '1';
                }, 100);
            } catch (e) {
                console.error('Coinzilla init error:', e);
            }
        }
    }
    
    detectAdBlock() {
        // Check if Coinzilla ads are blocked
        const testAd = document.createElement('div');
        testAd.className = 'coinzilla-ad';
        testAd.style.position = 'absolute';
        testAd.style.top = '-100px';
        testAd.style.left = '-100px';
        document.body.appendChild(testAd);
        
        setTimeout(() => {
            if (testAd.offsetHeight === 0 || !window.coinzilla) {
                this.adBlockDetected = true;
                this.showAdBlockMessage();
            }
            testAd.remove();
        }, 100);
    }
    
    showAdBlockMessage() {
        // Only show on certain pages
        const currentPath = window.location.pathname;
        if (currentPath === '/' || currentPath.includes('/p/')) return;
        
        const message = document.createElement('div');
        message.className = 'adblock-message';
        message.innerHTML = `
            <div class="adblock-content">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z"/>
                </svg>
                <span>We noticed you're using an ad blocker. We use crypto-friendly ads from Coinzilla to keep BinCrypt free. Consider allowlisting us!</span>
                <button onclick="this.parentElement.parentElement.remove()">✕</button>
            </div>
        `;
        
        setTimeout(() => {
            document.body.appendChild(message);
        }, 2000);
    }
    
    // Helper to create ad containers
    static createAdContainer(type) {
        const container = document.createElement('div');
        container.className = 'coinzilla-ad-container';
        container.setAttribute('data-ad-type', type);
        
        // Set dimensions based on ad type
        switch(type) {
            case 'banner728x90':
                container.style.minHeight = '90px';
                container.style.maxWidth = '728px';
                container.style.margin = '2rem auto';
                break;
            case 'sidebar300x600':
                container.style.minHeight = '600px';
                container.style.maxWidth = '300px';
                break;
            case 'rectangle300x250':
                container.style.minHeight = '250px';
                container.style.maxWidth = '300px';
                container.style.margin = '2rem auto';
                break;
            case 'mobile320x50':
                container.style.minHeight = '50px';
                container.style.maxWidth = '320px';
                container.style.margin = '1rem auto';
                break;
        }
        
        return container;
    }
}

// Global styles for Coinzilla ads
const coinzillaStyles = `
    .coinzilla-ad-container {
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: 8px;
        padding: 0.5rem;
        text-align: center;
        position: relative;
        overflow: hidden;
    }
    
    .ad-label {
        font-size: 0.75rem;
        color: var(--text-dim);
        text-transform: uppercase;
        letter-spacing: 0.05em;
        margin-bottom: 0.5rem;
    }
    
    .coinzilla-ad {
        display: flex;
        align-items: center;
        justify-content: center;
    }
    
    .adblock-message {
        position: fixed;
        bottom: 20px;
        right: 20px;
        max-width: 400px;
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: 8px;
        padding: 1rem;
        box-shadow: 0 4px 12px rgba(0,0,0,0.3);
        z-index: 1000;
        animation: slideIn 0.3s ease;
    }
    
    .adblock-content {
        display: flex;
        align-items: flex-start;
        gap: 0.75rem;
        font-size: 0.875rem;
        color: var(--text-dim);
    }
    
    .adblock-content svg {
        flex-shrink: 0;
        color: #f59e0b;
    }
    
    .adblock-content button {
        position: absolute;
        top: 0.5rem;
        right: 0.5rem;
        background: transparent;
        border: none;
        color: var(--text-dim);
        cursor: pointer;
        font-size: 1.2rem;
        padding: 0.25rem;
        line-height: 1;
    }
    
    .adblock-content button:hover {
        color: var(--text);
    }
    
    @keyframes slideIn {
        from {
            transform: translateX(100%);
            opacity: 0;
        }
        to {
            transform: translateX(0);
            opacity: 1;
        }
    }
    
    /* Mobile responsive */
    @media (max-width: 768px) {
        .coinzilla-ad-container[data-ad-type="sidebar300x600"] {
            display: none;
        }
        
        .adblock-message {
            left: 20px;
            right: 20px;
            max-width: none;
        }
    }
    
    /* Hide ads for premium users */
    body.premium-user .coinzilla-ad-container {
        display: none;
    }
`;

// Inject styles
const styleSheet = document.createElement('style');
styleSheet.textContent = coinzillaStyles;
document.head.appendChild(styleSheet);

// Initialize ads when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
        window.binCryptCoinzillaAds = new BinCryptCoinzillaAds();
    });
} else {
    window.binCryptCoinzillaAds = new BinCryptCoinzillaAds();
}