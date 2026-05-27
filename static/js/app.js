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

  function updateHeaderHeight() {
    const header = document.querySelector('.site-header');
    if (!header) return;
    document.documentElement.style.setProperty('--header-height', `${header.offsetHeight}px`);
  }

  function updateScheduleControlsHeight() {
    const controls = document.querySelector('.schedule-controls');
    if (!controls) return;
    document.documentElement.style.setProperty('--schedule-controls-height', `${controls.offsetHeight}px`);
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

  function initSchedulePicker() {
    const select = document.getElementById('schedule-team-select');
    const monthPicker = document.getElementById('schedule-month-picker');
    const prevMonth = document.getElementById('schedule-month-prev');
    const nextMonth = document.getElementById('schedule-month-next');
    const sections = Array.from(document.querySelectorAll('[data-schedule-team]'));
    if (!select || sections.length === 0) return;

    const teamIDs = sections.map((section) => section.dataset.scheduleTeam);
    const jumpToToday = (section) => {
      const candidates = Array.from(section.querySelectorAll('.schedule-agenda-day--today, [data-schedule-today="true"], .schedule-day--today'));
      const today = candidates.find((candidate) => candidate.offsetParent !== null) || candidates[0];
      if (!today) return;
      const scroll = () => {
        const block = window.matchMedia('(max-width: 920px)').matches ? 'start' : 'center';
        today.scrollIntoView({ block, behavior: 'smooth' });
      };
      requestAnimationFrame(scroll);
      setTimeout(scroll, 150);
      setTimeout(scroll, 500);
    };

    const monthElements = (section) => Array.from(section.querySelectorAll('[data-schedule-month]'));
    const monthHasGame = (month) => Boolean(month.querySelector('.schedule-day--game'));

    const populateMonths = (section) => {
      if (!monthPicker) return '';
      const months = monthElements(section);
      const values = months.map((month) => month.dataset.scheduleMonth || '').filter(Boolean);
      monthPicker.disabled = values.length === 0;
      if (prevMonth) prevMonth.disabled = values.length === 0;
      if (nextMonth) nextMonth.disabled = values.length === 0;
      if (values.length > 0) {
        monthPicker.min = values[0];
        monthPicker.max = values[values.length - 1];
      } else {
        monthPicker.removeAttribute('min');
        monthPicker.removeAttribute('max');
      }
      const currentWithGame = months.find((month) => month.dataset.currentMonth === 'true' && monthHasGame(month));
      const firstWithGame = months.find(monthHasGame);
      const current = currentWithGame || firstWithGame || months.find((month) => month.dataset.currentMonth === 'true') || months[0];
      const selected = current ? current.dataset.scheduleMonth : '';
      monthPicker.value = selected;
      return selected;
    };

    const showMonth = (section, monthKey, scrollToday) => {
      const months = monthElements(section);
      const selected = months.some((month) => month.dataset.scheduleMonth === monthKey)
        ? monthKey
        : (months[0] ? months[0].dataset.scheduleMonth : '');
      months.forEach((month) => {
        month.hidden = month.dataset.scheduleMonth !== selected;
      });
      if (monthPicker && selected) monthPicker.value = selected;
      if (scrollToday) jumpToToday(section);
    };

    const adjustMonth = (delta) => {
      if (!monthPicker || !monthPicker.value) return;
      const [year, month] = monthPicker.value.split('-').map(Number);
      if (!year || !month) return;
      const next = new Date(year, month - 1 + delta, 1);
      const value = `${next.getFullYear()}-${String(next.getMonth() + 1).padStart(2, '0')}`;
      if ((monthPicker.min && value < monthPicker.min) || (monthPicker.max && value > monthPicker.max)) return;
      monthPicker.value = value;
      const activeSection = sections.find((section) => !section.hidden);
      if (activeSection) showMonth(activeSection, value, true);
    };

    const showTeam = (teamID, updateHash, scrollToday) => {
      const selected = teamIDs.includes(teamID) ? teamID : teamIDs[0];
      select.value = selected;
      let activeSection = null;
      sections.forEach((section) => {
        const active = section.dataset.scheduleTeam === selected;
        section.hidden = !active;
        if (active) activeSection = section;
      });
      updateScheduleControlsHeight();
      if (updateHash) history.replaceState(null, '', `#${selected}`);
      if (activeSection) {
        const monthKey = populateMonths(activeSection);
        showMonth(activeSection, monthKey, scrollToday);
      }
    };

    const initial = window.location.hash.replace('#', '');
    const selected = teamIDs.includes(initial) ? initial : 'all';
    showTeam(selected, false, true);
    select.addEventListener('change', () => showTeam(select.value, true, true));
    if (monthPicker) {
      monthPicker.addEventListener('change', () => {
        const activeSection = sections.find((section) => !section.hidden);
        if (activeSection) showMonth(activeSection, monthPicker.value, true);
      });
    }
    if (prevMonth) prevMonth.addEventListener('click', () => adjustMonth(-1));
    if (nextMonth) nextMonth.addEventListener('click', () => adjustMonth(1));
    window.addEventListener('hashchange', () => showTeam(window.location.hash.replace('#', ''), false, true));
  }

  function ensureBaseballCurrentPlay(baseball) {
    let current = baseball.querySelector('.baseball-current-play');
    if (current) return current;

    current = document.createElement('div');
    current.className = 'baseball-current-play';
    const label = document.createElement('small');
    label.textContent = 'Current play';
    const text = document.createElement('strong');
    current.append(label, text);
    baseball.append(current);
    return current;
  }

  function ensureBaseballPlays(baseball) {
    let list = baseball.querySelector('.baseball-plays');
    if (list) return list;

    list = document.createElement('ul');
    list.className = 'baseball-plays';
    baseball.append(list);
    return list;
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

      if (game.Baseball.CurrentPlay) {
        const current = ensureBaseballCurrentPlay(baseball);
        const text = current.querySelector('strong');
        if (text) text.textContent = game.Baseball.CurrentPlay;
      }

      if (Array.isArray(game.Baseball.Plays) && game.Baseball.Plays.length > 0) {
        const list = ensureBaseballPlays(baseball);
        list.replaceChildren(...game.Baseball.Plays.map((play) => {
          const item = document.createElement('li');
          const inning = document.createElement('span');
          inning.textContent = play.Inning ? `${play.HalfInning || ''} ${play.Inning}`.trim() : '';
          const desc = document.createElement('strong');
          desc.textContent = play.Description || '';
          item.append(inning, desc);
          return item;
        }));
      }
    }
    if (changed) {
      card.classList.add('score-updated');
      setTimeout(() => card.classList.remove('score-updated'), 800);
    }
  }

  function scheduleScoreFor(game, phillyHome) {
    if (!['Live', 'Final'].includes(game.Status)) return '';
    const phillyScore = phillyHome ? game.HomeScore : game.AwayScore;
    const oppScore = phillyHome ? game.AwayScore : game.HomeScore;
    return `${phillyScore}-${oppScore}`;
  }

  function scheduleResultFor(game, phillyHome) {
    if (game.Status !== 'Final') return '';
    const phillyScore = phillyHome ? game.HomeScore : game.AwayScore;
    const oppScore = phillyHome ? game.AwayScore : game.HomeScore;
    if (phillyScore > oppScore) return 'W';
    if (phillyScore < oppScore) return 'L';
    return 'T';
  }

  function updateScheduleDOM(game) {
    document.querySelectorAll(`.schedule-game[data-game-id="${game.ID}"]`).forEach((item) => {
      const phillyHome = item.dataset.phillyHome === 'true';
      const result = item.querySelector('.schedule-result');
      const score = item.querySelector('.schedule-score');
      const status = item.querySelector('.schedule-status');
      const isLive = game.Status === 'Live';
      const isFinal = game.Status === 'Final';

      item.classList.toggle('schedule-game--live', isLive);
      item.classList.toggle('schedule-game--final', isFinal);

      const scoreText = scheduleScoreFor(game, phillyHome);
      if (score) {
        score.textContent = scoreText;
        score.hidden = !scoreText;
      }

      const resultText = scheduleResultFor(game, phillyHome);
      if (result) {
        result.textContent = resultText;
        result.hidden = !resultText;
        result.classList.remove('schedule-result--win', 'schedule-result--loss', 'schedule-result--tie');
        if (resultText === 'W') result.classList.add('schedule-result--win');
        if (resultText === 'L') result.classList.add('schedule-result--loss');
        if (resultText === 'T') result.classList.add('schedule-result--tie');
      }

      if (status) {
        status.textContent = isLive ? 'Live' : (isFinal ? 'Final' : (game.Status || 'Scheduled'));
      }
    });
  }

  function emit(name, detail) {
    document.dispatchEvent(new CustomEvent(`phillyGametime:${name}`, { detail }));
  }

  let recentRefreshQueued = false;

  function refreshRecentOnGameEnd() {
    if (recentRefreshQueued || !document.querySelector('.recent-list')) return;
    recentRefreshQueued = true;
    setTimeout(() => window.location.reload(), 2500);
  }

  function pollScores() {
    fetch('/api/scores')
      .then((response) => response.json())
      .then((games) => games.forEach((game) => {
        updateCardDOM(game);
        updateScheduleDOM(game);
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
        if (['score_update', 'game_start', 'game_end'].includes(eventName)) {
          updateCardDOM(game);
          updateScheduleDOM(game);
        }
        if (eventName === 'game_end') refreshRecentOnGameEnd();
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
  updateHeaderHeight();
  updateScheduleControlsHeight();
  window.addEventListener('resize', () => {
    updateHeaderHeight();
    updateScheduleControlsHeight();
  });
  initSchedulePicker();
  pollScores();
  setInterval(pollScores, POLL_INTERVAL);

  window.PhillyGametime = {
    on(eventName, callback) {
      document.addEventListener(`phillyGametime:${eventName}`, (event) => callback(event.detail));
    },
  };
})();
