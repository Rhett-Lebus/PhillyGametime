'use strict';

(function () {
  const POLL_INTERVAL = 30000;

  function updateCardDOM(game) {
    const card = document.querySelector(`[data-game-id="${game.ID}"]`);
    if (!card) return;

    const away = card.querySelector('.score-away');
    const home = card.querySelector('.score-home');
    const period = card.querySelector('.game-period');
    let changed = false;

    if (away && String(game.AwayScore) !== away.textContent) {
      away.textContent = game.AwayScore;
      changed = true;
    }
    if (home && String(game.HomeScore) !== home.textContent) {
      home.textContent = game.HomeScore;
      changed = true;
    }
    if (period && game.Status === 'Live') {
      period.textContent = `${game.Period} - ${game.TimeLeft}`;
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
  setInterval(pollScores, POLL_INTERVAL);

  window.PhillyGametime = {
    on(eventName, callback) {
      document.addEventListener(`phillyGametime:${eventName}`, (event) => callback(event.detail));
    },
  };
})();
