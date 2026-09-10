// Fix for sphinx-design cards :link: option not working on multiversion
document.addEventListener('DOMContentLoaded', function() {
    const cards = document.querySelectorAll('.sd-card');
    
    cards.forEach(card => {
        const link = card.querySelector('a');
        
        if (link) {
            card.style.cursor = 'pointer';
            
            card.addEventListener('click', function(e) {
                if (!e.target.closest('a')) {
                    link.click();
                }
            });
            
            card.addEventListener('mouseenter', function() {
                card.style.transform = 'translateY(-2px)';
                card.style.transition = 'transform 0.2s ease';
            });
            
            card.addEventListener('mouseleave', function() {
                card.style.transform = 'translateY(0)';
            });
        }
    });
});
