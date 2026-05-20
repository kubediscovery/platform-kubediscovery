/* ============================================================
   Synera Landing Page — script.js
   ============================================================ */

'use strict';

/* ---- Utility ---- */
const $ = (sel, ctx = document) => ctx.querySelector(sel);
const $$ = (sel, ctx = document) => [...ctx.querySelectorAll(sel)];

/* ---- Nav: scroll-aware style + hamburger ---- */
(function initNav() {
  const header    = $('#nav-header');
  const hamburger = $('#nav-hamburger');
  const navLinks  = $('#nav-links');
  let ticking = false;

  function updateNavStyle() {
    if (window.scrollY > 20) {
      header.classList.add('is-scrolled');
    } else {
      header.classList.remove('is-scrolled');
    }
  }

  window.addEventListener('scroll', () => {
    if (!ticking) {
      requestAnimationFrame(() => {
        updateNavStyle();
        ticking = false;
      });
      ticking = true;
    }
  }, { passive: true });

  updateNavStyle();

  hamburger.addEventListener('click', () => {
    const isOpen = navLinks.classList.toggle('is-open');
    hamburger.setAttribute('aria-expanded', String(isOpen));

    /* Prevent body scroll when menu is open on mobile */
    document.body.style.overflow = isOpen ? 'hidden' : '';
  });

  /* Close menu when a nav link is clicked */
  $$('.nav-link', navLinks).forEach(link => {
    link.addEventListener('click', () => {
      navLinks.classList.remove('is-open');
      hamburger.setAttribute('aria-expanded', 'false');
      document.body.style.overflow = '';
    });
  });

  /* Close menu when clicking outside */
  document.addEventListener('click', (e) => {
    if (
      navLinks.classList.contains('is-open') &&
      !navLinks.contains(e.target) &&
      !hamburger.contains(e.target)
    ) {
      navLinks.classList.remove('is-open');
      hamburger.setAttribute('aria-expanded', 'false');
      document.body.style.overflow = '';
    }
  });
})();

/* ---- Active nav link on scroll ---- */
(function initActiveNavLink() {
  const sections = $$('section[id], main[id]');
  const navLinks = $$('.nav-link');

  const observer = new IntersectionObserver(
    (entries) => {
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          const id = entry.target.getAttribute('id');
          navLinks.forEach(link => {
            const href = link.getAttribute('href');
            if (href === `#${id}`) {
              link.style.color = 'var(--color-orange)';
              link.style.fontWeight = '600';
            } else {
              link.style.color = '';
              link.style.fontWeight = '';
            }
          });
        }
      });
    },
    { rootMargin: '-50% 0px -50% 0px', threshold: 0 }
  );

  sections.forEach(s => observer.observe(s));
})();

/* ---- Footer year ---- */
(function updateFooterYear() {
  const yearEl = $('#footer-year');
  if (yearEl) yearEl.textContent = new Date().getFullYear();
})();

/* ---- AI Chat Widget ---- */
(function initAIChatWidget() {
  const fab         = $('#ai-chat-fab');
  const widget      = $('#ai-chat-widget');
  const closeBtns   = $$('#close-ai-chat, #open-ai-chat');
  const openBtn     = $('#open-ai-chat');
  const messages    = $('#ai-chat-messages');
  const form        = $('#ai-chat-form');
  const input       = $('#ai-chat-input');
  const fabIconOpen  = fab.querySelector('.ai-chat-fab-icon--open');
  const fabIconClose = fab.querySelector('.ai-chat-fab-icon--close');
  const suggestions  = $$('.ai-chat-suggestion');

  let isOpen = false;

  function openChat() {
    isOpen = true;
    widget.hidden = false;
    fabIconOpen.hidden = true;
    fabIconClose.hidden = false;
    fab.setAttribute('aria-label', 'Fechar chat');
    input.focus();
  }

  function closeChat() {
    isOpen = false;
    widget.hidden = true;
    fabIconOpen.hidden = false;
    fabIconClose.hidden = true;
    fab.setAttribute('aria-label', 'Abrir chat com IA');
  }

  fab.addEventListener('click', () => {
    if (isOpen) closeChat();
    else openChat();
  });

  if (openBtn) openBtn.addEventListener('click', openChat);

  $('#close-ai-chat').addEventListener('click', closeChat);

  /* Close on Escape */
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && isOpen) closeChat();
  });

  /* ---- Mock AI responses ---- */
  const botResponses = {
    'o que é platform engineering': 'Platform Engineering é a disciplina de criar e manter plataformas internas (IDPs) que abstraem a complexidade de infraestrutura para os times de desenvolvimento. O objetivo é aumentar a produtividade dos devs, reduzir o atrito e garantir governança. Ferramentas como Backstage, Crossplane e Kubernetes são pilares dessa prática.',
    'como funciona o diagnóstico gratuito': 'Nosso diagnóstico gratuito é uma conversa inicial de 30–60 minutos onde entendemos seu stack atual, principais dores e objetivos. A partir daí, a Synera apresenta um mapa de oportunidades com prioridades e estimativas de esforço. Sem compromisso!',
    'quais treinamentos vocês oferecem': 'Oferecemos treinamentos in-company em: Kubernetes Essentials, GitOps & CI/CD Avançado, Platform Engineering na Prática e Observabilidade com OpenTelemetry. Todos são customizáveis ao contexto do seu time. Posso te conectar com um especialista para montar a trilha ideal!',
    'default': 'Boa pergunta! Para uma resposta mais aprofundada, recomendo falar diretamente com nossa equipe pelo WhatsApp. Eles estão prontos para entender seu contexto e propor a melhor solução. Posso ajudar com mais alguma coisa?',
  };

  function normalizeText(text) {
    return text.toLowerCase()
      .normalize('NFD')
      .replace(/[\u0300-\u036f]/g, '')
      .trim();
  }

  function getBotResponse(userMessage) {
    const normalized = normalizeText(userMessage);
    for (const [key, response] of Object.entries(botResponses)) {
      if (key !== 'default' && normalized.includes(normalizeText(key))) {
        return response;
      }
    }
    return botResponses['default'];
  }

  function appendMessage(text, type = 'bot') {
    const div = document.createElement('div');
    div.className = `ai-chat-message ai-chat-message--${type}`;
    const p = document.createElement('p');
    p.textContent = text;
    div.appendChild(p);
    messages.appendChild(div);
    messages.scrollTop = messages.scrollHeight;
    return div;
  }

  function showTyping() {
    const div = document.createElement('div');
    div.className = 'ai-chat-typing';
    div.innerHTML = '<span></span><span></span><span></span>';
    messages.appendChild(div);
    messages.scrollTop = messages.scrollHeight;
    return div;
  }

  function sendMessage(text) {
    if (!text.trim()) return;

    /* Remove suggestions after first user message */
    const suggestionsEl = $('.ai-chat-suggestions', messages);
    if (suggestionsEl) suggestionsEl.remove();

    appendMessage(text, 'user');
    input.value = '';

    const typingEl = showTyping();
    const delay = 800 + Math.random() * 600;

    setTimeout(() => {
      typingEl.remove();
      appendMessage(getBotResponse(text), 'bot');
    }, delay);
  }

  form.addEventListener('submit', (e) => {
    e.preventDefault();
    sendMessage(input.value);
  });

  suggestions.forEach(btn => {
    btn.addEventListener('click', () => {
      sendMessage(btn.textContent);
    });
  });
})();

/* ---- Scroll animations (Intersection Observer) ---- */
(function initScrollAnimations() {
  /* Skip if user prefers reduced motion */
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;

  const style = document.createElement('style');
  style.textContent = `
    .anim-fade-up {
      opacity: 0;
      transform: translateY(24px);
      transition: opacity 0.5s ease, transform 0.5s ease;
    }
    .anim-fade-up.is-visible {
      opacity: 1;
      transform: none;
    }
  `;
  document.head.appendChild(style);

  const animTargets = [
    ...$$('.pillar-card'),
    ...$$('.service-card'),
    ...$$('.training-card'),
    ...$$('.contact-card'),
  ];

  animTargets.forEach((el, i) => {
    el.classList.add('anim-fade-up');
    el.style.transitionDelay = `${(i % 3) * 80}ms`;
  });

  const observer = new IntersectionObserver(
    (entries) => {
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          entry.target.classList.add('is-visible');
          observer.unobserve(entry.target);
        }
      });
    },
    { threshold: 0.1, rootMargin: '0px 0px -40px 0px' }
  );

  animTargets.forEach(el => observer.observe(el));
})();
