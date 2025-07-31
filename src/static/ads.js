// BinCrypt AdSense Integration
// Handles ad loading, placement, and user experience

class BinCryptAds {
    constructor() {
        this.adClient = null;
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
            }
        }
    }
    
    async init() {
        // Fetch ad configuration from server
        try {
            const response = await fetch('/api/config');
            const config = await response.json();
            
            if (config.adClient && config.adClient !== 'ca-pub-XXXXXXXXXXXXXXXX') {
                this.adClient = config.adClient;
                
                // Load AdSense script
                this.loadAdSenseScript();
                
                // Setup intersection observer for lazy loading
                this.setupLazyLoading();
                
                // Detect ad blockers
                setTimeout(() => this.detectAdBlock(), 1000);
            } else {
                console.info('AdSense not configured');
            }
        } catch (err) {
            console.error('Failed to fetch ad configuration:', err);
        }
    }
    
    loadAdSenseScript() {
        if (document.getElementById('adsense-script')) return;
        
        const script = document.createElement('script');
        script.id = 'adsense-script';
        script.async = true;
        script.src = `https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=${this.adClient}`;
        script.crossOrigin = 'anonymous';
        script.onerror = () => {
            this.adBlockDetected = true;
            this.showAdBlockMessage();
        };
        document.head.appendChild(script);
        
        this.adsLoaded = true;
    }
    
    setupLazyLoading() {
        // Use Intersection Observer to load ads when they're about to be viewed
        const adContainers = document.querySelectorAll('.ad-container:not(.ad-loaded)');
        
        if ('IntersectionObserver' in window) {
            const imageObserver = new IntersectionObserver((entries) => {
                entries.forEach(entry => {
                    if (entry.isIntersecting) {
                        this.loadAd(entry.target);
                        imageObserver.unobserve(entry.target);
                    }
                });
            }, {
                rootMargin: '200px 0px' // Start loading 200px before the ad comes into view
            });
            
            adContainers.forEach(container => imageObserver.observe(container));
        } else {
            // Fallback for older browsers
            adContainers.forEach(container => this.loadAd(container));
        }
    }
    
    loadAd(container) {
        if (container.classList.contains('ad-loaded') || this.isPremiumUser || !this.adClient) return;
        
        const adFormat = container.getAttribute('data-ad-format') || 'auto';
        const adSlot = container.getAttribute('data-ad-slot');
        
        // Create ad element
        const ins = document.createElement('ins');
        ins.className = 'adsbygoogle';
        ins.style.display = 'block';
        ins.setAttribute('data-ad-client', this.adClient);
        ins.setAttribute('data-ad-slot', adSlot);
        ins.setAttribute('data-ad-format', adFormat);
        ins.setAttribute('data-full-width-responsive', 'true');
        
        // Add label
        const label = document.createElement('div');
        label.className = 'ad-label';
        label.textContent = 'Advertisement';
        
        container.appendChild(label);
        container.appendChild(ins);
        container.classList.add('ad-loaded');
        
        // Animate in
        container.style.opacity = '0';
        container.style.transition = 'opacity 0.3s ease-in';
        
        // Push ad
        try {
            (window.adsbygoogle = window.adsbygoogle || []).push({});
            setTimeout(() => {
                container.style.opacity = '1';
            }, 100);
        } catch (e) {
            console.error('AdSense error:', e);
        }
    }
    
    detectAdBlock() {
        // Simple ad block detection
        const testAd = document.createElement('div');
        testAd.innerHTML = '&nbsp;';
        testAd.className = 'adsbox';
        testAd.style.position = 'absolute';
        testAd.style.top = '-100px';
        testAd.style.left = '-100px';
        document.body.appendChild(testAd);
        
        setTimeout(() => {
            if (testAd.offsetHeight === 0) {
                this.adBlockDetected = true;
                this.showAdBlockMessage();
            }
            testAd.remove();
        }, 100);
    }
    
    showAdBlockMessage() {
        // Only show on certain pages, not on the main paste creation
        const currentPath = window.location.pathname;
        if (currentPath === '/' || currentPath.includes('/p/')) return;
        
        const message = document.createElement('div');
        message.className = 'adblock-message';
        message.innerHTML = `
            <div class="adblock-content">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z"/>
                </svg>
                <span>Hey there! We noticed you're using an ad blocker. We respect your choice, but ads help keep BinCrypt free for everyone. Consider disabling it to support us!</span>
                <button onclick="this.parentElement.parentElement.remove()">✕</button>
            </div>
        `;
        
        // Add to page after a delay
        setTimeout(() => {
            document.body.appendChild(message);
        }, 2000);
    }
    
    // Helper to create ad containers with proper sizing
    static createAdContainer(type, slot) {
        const container = document.createElement('div');
        container.className = 'ad-container';
        container.setAttribute('data-ad-slot', slot);
        
        switch(type) {
            case 'banner':
                container.setAttribute('data-ad-format', 'horizontal');
                container.style.minHeight = '90px';
                container.style.maxWidth = '728px';
                container.style.margin = '2rem auto';
                break;
            case 'sidebar':
                container.setAttribute('data-ad-format', 'vertical');
                container.style.minHeight = '600px';
                container.style.maxWidth = '300px';
                break;
            case 'rectangle':
                container.setAttribute('data-ad-format', 'rectangle');
                container.style.minHeight = '250px';
                container.style.maxWidth = '300px';
                container.style.margin = '2rem auto';
                break;
            case 'mobile-banner':
                container.setAttribute('data-ad-format', 'horizontal');
                container.style.minHeight = '50px';
                container.style.maxWidth = '320px';
                container.style.margin = '1rem auto';
                break;
        }
        
        return container;
    }
}

// Global styles for ads
const adStyles = `
    .ad-container {
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: 8px;
        padding: 0.5rem;
        text-align: center;
        position: relative;
    }
    
    .ad-label {
        font-size: 0.75rem;
        color: var(--text-dim);
        text-transform: uppercase;
        letter-spacing: 0.05em;
        margin-bottom: 0.5rem;
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
        .ad-container[data-ad-format="vertical"] {
            display: none;
        }
        
        .adblock-message {
            left: 20px;
            right: 20px;
            max-width: none;
        }
    }
    
    /* Hide ads for premium users */
    body.premium-user .ad-container {
        display: none;
    }
`;

// Inject styles
const styleSheet = document.createElement('style');
styleSheet.textContent = adStyles;
document.head.appendChild(styleSheet);

// Initialize ads when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
        window.binCryptAds = new BinCryptAds();
    });
} else {
    window.binCryptAds = new BinCryptAds();
}