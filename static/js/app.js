'use strict';

(function () {
  const POLL_INTERVAL = 30000;
  const THEME_KEY = 'phillyGametimeTheme';
  const DEFAULT_THEME = 'neon';

  function getStoredTheme() {
    try {
      return localStorage.getItem(THEME_KEY) || DEFAULT_THEME;
    } catch {
      return DEFAULT_THEME;
    }
  }

  function storeTheme(theme) {
    try {
      localStorage.setItem(THEME_KEY, theme);
    } catch {}
  }

  function applyTheme(theme) {
    document.documentElement.dataset.theme = theme || DEFAULT_THEME;
  }

  function initThemePicker() {
    const select = document.getElementById('theme-select');
    if (!select) return;

    const currentTheme = getStoredTheme();
    select.value = currentTheme;
    applyTheme(currentTheme);

    select.addEventListener('change', () => {
      applyTheme(select.value);
      storeTheme(select.value);
    });
  }

  function updateCardDOM(game) {
    const card = document.querySelector(`[data-game-id="${game.ID}"]`);
    if (!card) return;

    const away = card.querySelector('.score-away');
    const home = card.querySelector('.score-home');
    const badge = card.querySelector('.badge');
    const period = card.querySelector('.game-period');
    let changed = false;

    const statusClass = `game-card--${String(game.Status || '').toLowerCase()}`;
    ['game-card--scheduled', 'game-card--live', 'game-card--final', 'game-card--delayed', 'game-card--postponed', 'game-card--cancelled']
      .forEach((className) => card.classList.toggle(className, className === statusClass));

    if (badge) {
      ['badge--scheduled', 'badge--live', 'badge--final', 'badge--delayed', 'badge--postponed', 'badge--cancelled']
        .forEach((className) => badge.classList.remove(className));
      badge.classList.add(`badge--${String(game.Status || 'Scheduled').toLowerCase()}`);
      badge.textContent = game.Status || 'Scheduled';
    }

    if (away && String(game.AwayScore) !== away.textContent) {
      away.textContent = game.AwayScore;
      changed = true;
    }
    if (home && String(game.HomeScore) !== home.textContent) {
      home.textContent = game.HomeScore;
      changed = true;
    }
    if (period) {
      if (game.Status === 'Live') {
        period.textContent = game.TimeLeft ? `${game.Period} - ${game.TimeLeft}` : game.Period;
      } else if (['Delayed', 'Postponed', 'Cancelled', 'Final'].includes(game.Status) && game.Period) {
        period.textContent = game.Period;
      }
    }

    const baseball = card.querySelector('.baseball-live');
    if (baseball && game.Baseball) {
      const states = {
        first: game.Baseball.OnFirst,
        second: game.Baseball.OnSecond,
        third: game.Baseball.OnThird,
      };
      Object.entries(states).forEach(([baseName, occupied]) => {
        const base = baseball.querySelector(`.base--${baseName}`);
        if (base) base.classList.toggle('base--occupied', Boolean(occupied));
      });

      const count = baseball.querySelectorAll('.baseball-count strong');
      if (count[0]) count[0].textContent = game.Baseball.Outs;
      if (count[1]) count[1].textContent = game.Baseball.Balls;
      if (count[2]) count[2].textContent = game.Baseball.Strikes;

      const players = baseball.querySelectorAll('.baseball-players strong');
      if (players[0] && game.Baseball.Batter) players[0].textContent = game.Baseball.Batter;
      if (players[1] && game.Baseball.Pitcher) players[1].textContent = game.Baseball.Pitcher;

      const pitcherK = baseball.querySelector('.baseball-pitcher-k');
      if (pitcherK) pitcherK.textContent = game.Baseball.PitcherStrikeouts ? `${game.Baseball.PitcherStrikeouts} K` : '';
    }
    if (changed) {
      card.classList.add('score-updated');
      setTimeout(() => card.classList.remove('score-updated'), 800);
    }
  }

  function emit(name, detail) {
    document.dispatchEvent(new CustomEvent(`phillyGametime:${name}`, { detail }));
  }

  function pollScores() {
    fetch('/api/scores')
      .then((response) => response.json())
      .then((games) => games.forEach((game) => {
        updateCardDOM(game);
        emit('score_update', game);
      }))
      .catch(() => {});
  }

  function connectEvents() {
    if (!window.EventSource) return false;

    const source = new EventSource('/events');
    const eventNames = ['score_update', 'game_start', 'game_end', 'goal_scored', 'touchdown', 'home_run', 'basket'];

    eventNames.forEach((eventName) => {
      source.addEventListener(eventName, (event) => {
        const game = JSON.parse(event.data);
        if (eventName === 'score_update') updateCardDOM(game);
        emit(eventName, game);
      });
    });

    source.onerror = () => {
      source.close();
      setTimeout(connectEvents, 10000);
    };
    return true;
  }

  connectEvents();
  initThemePicker();
  setInterval(pollScores, POLL_INTERVAL);

  window.PhillyGametime = {
    on(eventName, callback) {
      document.addEventListener(`phillyGametime:${eventName}`, (event) => callback(event.detail));
    },
  };
})();
