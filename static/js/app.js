'use strict';

(function () {
  const POLL_INTERVAL = 30000;
  const THEME_KEY = 'phillyGametimeTheme';
  const DEFAULT_THEME = 'neon';
  const lineupCache = new Map();

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
    const todayKey = () => {
      const now = new Date();
      const year = now.getFullYear();
      const month = String(now.getMonth() + 1).padStart(2, '0');
      const day = String(now.getDate()).padStart(2, '0');
      return `${year}-${month}-${day}`;
    };
    const visible = (element) => Boolean(element && element.offsetParent !== null);
    const scheduleDate = (element) => element ? (element.dataset.scheduleDate || '') : '';
    const gameDays = (section) => Array.from(section.querySelectorAll('.schedule-day--game[data-schedule-date]'));
    const nextGameDay = (section) => {
      const today = todayKey();
      return gameDays(section).find((day) => scheduleDate(day) >= today) || null;
    };
    const jumpToScheduleTarget = (section) => {
      const today = todayKey();
      const isMobile = window.matchMedia('(max-width: 920px)').matches;
      const visibleToday = Array.from(section.querySelectorAll('.schedule-agenda-day--today, [data-schedule-today="true"], .schedule-day--today'))
        .find(visible);
      const visibleUpcoming = Array.from(section.querySelectorAll('.schedule-agenda-day[data-schedule-date], .schedule-day--game[data-schedule-date]'))
        .find((day) => visible(day) && scheduleDate(day) >= today);
      const visibleGame = Array.from(section.querySelectorAll('.schedule-agenda-day[data-schedule-date], .schedule-day--game[data-schedule-date]'))
        .find(visible);
      const visibleMonth = monthElements(section).find(visible);
      const target = visibleToday || visibleUpcoming || visibleGame || visibleMonth;
      if (!target) return;
      const scroll = () => {
        const block = isMobile ? 'start' : 'center';
        target.scrollIntoView({ block, behavior: 'smooth' });
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
      const upcoming = nextGameDay(section);
      const upcomingMonth = upcoming ? upcoming.closest('[data-schedule-month]') : null;
      const currentWithGame = months.find((month) => month.dataset.currentMonth === 'true' && monthHasGame(month));
      const firstWithGame = months.find(monthHasGame);
      const current = upcomingMonth || currentWithGame || firstWithGame || months.find((month) => month.dataset.currentMonth === 'true') || months[0];
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
      if (scrollToday) jumpToScheduleTarget(section);
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

  function initLeagueStandingsPicker() {
    const select = document.getElementById('league-sport-select');
    const sections = Array.from(document.querySelectorAll('[data-league-sport]'));
    if (!select || sections.length === 0) return;
    const standingsPanel = select.closest('.league-standings-panel');
    let jumpToken = 0;
    let jumping = false;

    const setFloatingControls = (stuck) => {
      if (!standingsPanel || jumping) return;
      standingsPanel.classList.toggle('controls-stuck', stuck);
      if (!stuck) return;

      const rect = standingsPanel.getBoundingClientRect();
      standingsPanel.style.setProperty('--league-controls-left', `${Math.max(rect.left, 10)}px`);
      standingsPanel.style.setProperty('--league-controls-width', `${Math.min(rect.width, window.innerWidth - 20)}px`);
    };

    const updateFloatingControls = () => {
      if (!standingsPanel || jumping) return;
      const header = document.querySelector('.site-header');
      const headerHeight = header ? header.offsetHeight : 80;
      const rect = standingsPanel.getBoundingClientRect();
      const stuck = rect.top <= headerHeight + 8 && rect.bottom > headerHeight + 180;

      setFloatingControls(stuck);
    };

    const activeScopePanel = () => {
      const section = sections.find((item) => !item.hidden);
      return section ? section.querySelector('[data-league-scope-panel]:not([hidden])') : null;
    };

    const jumpToPhillyRow = (scopePanel = activeScopePanel()) => {
      const token = ++jumpToken;
      jumping = true;
      if (standingsPanel) {
        jumping = false;
        setFloatingControls(true);
        jumping = true;
      }

      const scroll = () => {
        const targetScopePanel = scopePanel && !scopePanel.hidden ? scopePanel : activeScopePanel();
        if (token !== jumpToken || !targetScopePanel) return;
        const row = targetScopePanel.querySelector('.standings-row--philly');
        if (!row) return;

        const header = document.querySelector('.site-header');
        const headerHeight = header ? header.offsetHeight : 80;
        const controls = standingsPanel ? standingsPanel.querySelector('.league-controls') : null;
        const scopeControl = standingsPanel ? standingsPanel.querySelector('.league-standings:not([hidden]) .league-scope-control') : null;
        const controlsHeight = (controls ? controls.offsetHeight : 0) + (scopeControl ? scopeControl.offsetHeight : 0);
        const rect = row.getBoundingClientRect();
        const top = Math.max(0, window.scrollY + rect.top - headerHeight - controlsHeight - 75);

        window.scrollTo({ top, behavior: 'smooth' });
        setTimeout(() => {
          if (token !== jumpToken) return;
          jumping = false;
          updateFloatingControls();
        }, 560);
      };
      requestAnimationFrame(scroll);
      setTimeout(scroll, 220);
    };

    const showScope = (section, scope, jump = true) => {
      const panels = Array.from(section.querySelectorAll('[data-league-scope-panel]'));
      const scopeSelect = section.querySelector('[data-league-scope-select]');
      const selected = panels.some((panel) => panel.dataset.leagueScopePanel === scope)
        ? scope
        : (panels[0] ? panels[0].dataset.leagueScopePanel : '');

      if (scopeSelect && selected) scopeSelect.value = selected;
      let activePanel = null;
      panels.forEach((panel) => {
        const active = panel.dataset.leagueScopePanel === selected;
        panel.hidden = !active;
        if (active) activePanel = panel;
      });
      if (jump) jumpToPhillyRow(activePanel);
      return activePanel;
    };

    const showSport = (sport, jump = true) => {
      let activePanel = null;
      sections.forEach((section) => {
        const active = section.dataset.leagueSport === sport;
        section.hidden = !active;
        if (active) {
          const scopeSelect = section.querySelector('[data-league-scope-select]');
          const firstPanel = section.querySelector('[data-league-scope-panel]');
          activePanel = showScope(section, scopeSelect ? scopeSelect.value : (firstPanel ? firstPanel.dataset.leagueScopePanel : ''), false);
        }
      });
      if (jump) jumpToPhillyRow(activePanel);
    };

    sections.forEach((section) => {
      const scopeSelect = section.querySelector('[data-league-scope-select]');
      if (scopeSelect) scopeSelect.addEventListener('change', () => {
        showScope(section, scopeSelect.value);
        updateFloatingControls();
      });
    });
    select.addEventListener('change', () => {
      showSport(select.value);
      updateFloatingControls();
    });
    window.addEventListener('scroll', updateFloatingControls, { passive: true });
    window.addEventListener('resize', updateFloatingControls);
    showSport(select.value, false);
    updateFloatingControls();
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
    rememberGameLineup(game);

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
    if (baseball && (game.Status !== 'Live' || !game.Baseball)) {
      baseball.remove();
    } else if (baseball && game.Baseball) {
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
    rememberGameLineup(game);
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

  function rememberGameLineup(game) {
    if (!game || !game.ID || !game.Lineup) return;
    lineupCache.set(game.ID, { Available: true, Lineup: game.Lineup });
    markLineupPosted(game.ID);
  }

  function markLineupPosted(gameID) {
    document.querySelectorAll(`[data-lineup-game="${gameID}"]`).forEach((button) => {
      button.classList.add('lineup-button--ready');
      button.textContent = 'Lineup Posted';
    });
  }

  function teamLabel(team) {
    if (!team) return '';
    return [team.City, team.Name].filter(Boolean).join(' ').trim() || team.Name || team.Abbr || '';
  }

  function lineupRows(entries) {
    if (!Array.isArray(entries) || entries.length === 0) {
      const empty = document.createElement('p');
      empty.className = 'lineup-empty';
      empty.textContent = 'Not posted yet.';
      return [empty];
    }
    const list = document.createElement('ol');
    list.className = 'lineup-list';
    entries.forEach((entry, index) => {
      const item = document.createElement('li');
      const order = document.createElement('span');
      order.className = 'lineup-order';
      order.textContent = entry.Order || index + 1;
      const name = document.createElement('strong');
      name.textContent = entry.Name || 'TBD';
      const position = document.createElement('span');
      position.className = 'lineup-position';
      position.textContent = entry.Position || '';
      item.append(order, name, position);
      list.append(item);
    });
    return [list];
  }

  function pitcherLabel(pitcher) {
    if (!pitcher || !pitcher.Name) return 'TBD';
    const hand = pitcher.Handedness ? ` ${pitcher.Handedness}HP` : '';
    return `${pitcher.Name}${hand}`;
  }

  function pitcherCard(team, pitcher, extraClass = '') {
    const card = document.createElement('div');
    card.className = `lineup-pitcher-card${extraClass ? ` ${extraClass}` : ''}`;
    if (team && team.Primary) card.style.setProperty('--lineup-team-color', team.Primary);
    if (team && team.Secondary) card.style.setProperty('--lineup-team-accent', team.Secondary);

    const label = document.createElement('small');
    label.textContent = 'Starting pitcher';
    const name = document.createElement('strong');
    name.textContent = pitcherLabel(pitcher);
    card.append(label, name);
    return card;
  }

  function pitcherHasName(pitcher) {
    return Boolean(pitcher && pitcher.Name);
  }

  function renderPitchersDesktop(lineup) {
    if (!lineup || (!pitcherHasName(lineup.AwayPitcher) && !pitcherHasName(lineup.HomePitcher))) return null;
    const wrap = document.createElement('section');
    wrap.className = 'lineup-pitchers lineup-pitchers--desktop';
    wrap.append(
      pitcherCard(lineup.AwayTeam, lineup.AwayPitcher),
      pitcherCard(lineup.HomeTeam, lineup.HomePitcher),
    );
    return wrap;
  }

  function renderLineup(modal, payload) {
    const body = modal.querySelector('.lineup-modal__body');
    if (!body) return;
    body.replaceChildren();

    if (!payload || !payload.Available || !payload.Lineup) {
      const message = document.createElement('p');
      message.className = 'lineup-empty';
      message.textContent = (payload && payload.Message) || 'Lineup has not been posted yet.';
      body.append(message);
      return;
    }

    const lineup = payload.Lineup;
    if (lineup.AwayTeam && lineup.AwayTeam.Primary) modal.style.setProperty('--lineup-away-color', lineup.AwayTeam.Primary);
    if (lineup.HomeTeam && lineup.HomeTeam.Primary) modal.style.setProperty('--lineup-home-color', lineup.HomeTeam.Primary);
    const pitchers = renderPitchersDesktop(lineup);
    if (pitchers) body.append(pitchers);
    [
      { team: lineup.AwayTeam, pitcher: lineup.AwayPitcher, entries: lineup.Away },
      { team: lineup.HomeTeam, pitcher: lineup.HomePitcher, entries: lineup.Home },
    ].forEach(({ team, pitcher, entries }) => {
      const section = document.createElement('section');
      section.className = 'lineup-team';
      if (team && team.Primary) section.style.setProperty('--lineup-team-color', team.Primary);
      if (team && team.Secondary) section.style.setProperty('--lineup-team-accent', team.Secondary);
      const heading = document.createElement('h3');
      if (team && team.LogoURL) {
        const logo = document.createElement('img');
        logo.src = team.LogoURL;
        logo.alt = `${teamLabel(team)} logo`;
        heading.append(logo);
      }
      const name = document.createElement('span');
      name.textContent = teamLabel(team);
      heading.append(name);
      section.append(heading);
      if (pitcherHasName(pitcher)) section.append(pitcherCard(team, pitcher, 'lineup-pitcher-card--mobile'));
      section.append(...lineupRows(entries));
      body.append(section);
    });
  }

  function createLineupModal() {
    const modal = document.createElement('div');
    modal.className = 'lineup-modal';
    modal.hidden = true;
    modal.innerHTML = `
      <div class="lineup-modal__backdrop" data-lineup-close></div>
      <section class="lineup-modal__panel" role="dialog" aria-modal="true" aria-labelledby="lineup-modal-title">
        <header class="lineup-modal__header">
          <h2 id="lineup-modal-title">Lineup</h2>
          <button type="button" class="lineup-modal__close" data-lineup-close aria-label="Close lineup">&times;</button>
        </header>
        <div class="lineup-modal__body"></div>
      </section>
    `;
    modal.addEventListener('click', (event) => {
      if (event.target.closest('[data-lineup-close]')) closeLineupModal(modal);
    });
    document.body.append(modal);
    return modal;
  }

  function closeLineupModal(modal = document.querySelector('.lineup-modal')) {
    if (!modal) return;
    modal.hidden = true;
    document.body.classList.remove('lineup-modal-open');
  }

  function showLineupLoading(modal, title) {
    const heading = modal.querySelector('#lineup-modal-title');
    const body = modal.querySelector('.lineup-modal__body');
    modal.style.removeProperty('--lineup-away-color');
    modal.style.removeProperty('--lineup-home-color');
    if (heading) heading.textContent = title || 'Lineup';
    if (body) {
      const loading = document.createElement('p');
      loading.className = 'lineup-empty';
      loading.textContent = 'Checking lineup...';
      body.replaceChildren(loading);
    }
    modal.hidden = false;
    document.body.classList.add('lineup-modal-open');
  }

  function initLineupButtons() {
    if (!document.querySelector('[data-lineup-game]')) return;
    const modal = createLineupModal();
    document.addEventListener('click', (event) => {
      const button = event.target.closest('[data-lineup-game]');
      if (!button) return;
      const gameID = button.dataset.lineupGame;
      if (!gameID) return;

      showLineupLoading(modal, button.dataset.lineupTitle || 'Lineup');
      if (lineupCache.has(gameID)) {
        renderLineup(modal, lineupCache.get(gameID));
        return;
      }

      fetch(`/api/games/${encodeURIComponent(gameID)}/lineup`)
        .then((response) => response.ok ? response.json() : Promise.reject(response))
        .then((payload) => {
          lineupCache.set(gameID, payload);
          if (payload && payload.Available) markLineupPosted(gameID);
          renderLineup(modal, payload);
        })
        .catch(() => {
          renderLineup(modal, { Available: false, Message: 'Lineup is unavailable right now.' });
        });
    });
    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') closeLineupModal(modal);
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
  initLeagueStandingsPicker();
  initLineupButtons();
  pollScores();
  setInterval(pollScores, POLL_INTERVAL);

  window.PhillyGametime = {
    on(eventName, callback) {
      document.addEventListener(`phillyGametime:${eventName}`, (event) => callback(event.detail));
    },
  };
})();
