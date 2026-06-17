'use strict';

(function () {
  const SCORE_LIVE_POLL_INTERVAL = 15 * 1000;
  const SCORE_IDLE_POLL_INTERVAL = 60 * 1000;
  const WORLD_CUP_ACTIVE_POLL_INTERVAL = 15 * 1000;
  const WORLD_CUP_IDLE_POLL_INTERVAL = 2 * 60 * 1000;
  const THEME_KEY = 'phillyGametimeTheme';
  const DEFAULT_THEME = 'neon';
  const LINEUP_PARTIAL_TTL = 2 * 60 * 1000;
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
    const teamSportMap = new Map([
      ['21', 'NFL'],
      ['eagles', 'NFL'],
      ['15', 'NHL'],
      ['flyers', 'NHL'],
      ['22', 'MLB'],
      ['phillies', 'MLB'],
      ['20', 'NBA'],
      ['sixers', 'NBA'],
      ['76ers', 'NBA'],
      ['10739', 'MLS'],
      ['union', 'MLS'],
    ]);
    let jumpToken = 0;
    let jumping = false;

    const sportValues = Array.from(select.options).map((option) => option.value);

    const sportFromURL = () => {
      const params = new URLSearchParams(window.location.search);
      const sport = (params.get('sport') || '').toUpperCase();
      if (sportValues.includes(sport)) return sport;

      const team = (params.get('team') || '').trim().toLowerCase();
      const teamSport = teamSportMap.get(team);
      return sportValues.includes(teamSport) ? teamSport : '';
    };

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
    const initialSport = sportFromURL();
    if (initialSport) select.value = initialSport;
    showSport(select.value, !!initialSport);
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
        period.hidden = false;
        period.textContent = game.TimeLeft ? `${game.Period} - ${game.TimeLeft}` : game.Period;
      } else if (['Delayed', 'Postponed', 'Cancelled', 'Final'].includes(game.Status) && game.Period) {
        period.hidden = game.Status === 'Final' && game.Period === 'Final';
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
    updateSoccerPulse(card, game.Soccer);
    if (changed) {
      card.classList.add('score-updated');
      setTimeout(() => card.classList.remove('score-updated'), 800);
    }
  }

  function soccerStatValue(value) {
    return value === undefined || value === null || value === '' ? '-' : value;
  }

  function soccerCardValue(value) {
    if (value === undefined || value === null) return '';
    const text = String(value).trim();
    if (!text) return '';
    const numeric = Number.parseInt(text, 10);
    if (Number.isFinite(numeric) && numeric <= 0) return '';
    return text;
  }

  function updateSoccerPulse(card, soccer) {
    if (!card || !soccer || !card.querySelector('[data-soccer-stat], [data-soccer-card]')) return;
    const setText = (selector, value) => {
      const node = card.querySelector(selector);
      if (node) node.textContent = value;
    };
    const setCard = (selector, value) => {
      const node = card.querySelector(selector);
      if (!node) return;
      const text = soccerCardValue(value);
      node.textContent = text;
      const wrapper = node.closest('span');
      if (wrapper) wrapper.hidden = !text;
    };
    const away = soccer.AwayStats || {};
    const home = soccer.HomeStats || {};
    setText('[data-soccer-stat="away-shots"]', soccerStatValue(away.Shots));
    setText('[data-soccer-stat="home-shots"]', soccerStatValue(home.Shots));
    setText('[data-soccer-stat="away-target"]', soccerStatValue(away.ShotsOnTarget));
    setText('[data-soccer-stat="home-target"]', soccerStatValue(home.ShotsOnTarget));
    setCard('[data-soccer-card="away-yellow"]', away.YellowCards);
    setCard('[data-soccer-card="away-red"]', away.RedCards);
    setCard('[data-soccer-card="home-yellow"]', home.YellowCards);
    setCard('[data-soccer-card="home-red"]', home.RedCards);
    updateSoccerPossession(card, soccer);
  }

  function updateSoccerPossession(card, soccer) {
    const away = soccer && soccer.AwayStats ? soccer.AwayStats.Possession : '';
    const home = soccer && soccer.HomeStats ? soccer.HomeStats.Possession : '';
    let wrap = card.querySelector('[data-soccer-possession]');
    if (!away || !home) {
      if (wrap) wrap.remove();
      return;
    }
    if (!wrap) {
      wrap = document.createElement('div');
      wrap.className = card.classList.contains('world-cup-match') ? 'soccer-possession soccer-possession--world-cup' : 'soccer-possession';
      wrap.setAttribute('data-soccer-possession', '');
      const label = document.createElement('small');
      label.textContent = 'Possession';
      wrap.append(
        label,
        soccerPossessionTeamRow(card, 'away'),
        soccerPossessionBar(),
        soccerPossessionTeamRow(card, 'home'),
      );
      const anchor = card.querySelector('.live-matchup, .world-cup-match__teams');
      if (anchor) anchor.insertAdjacentElement('afterend', wrap);
      else card.append(wrap);
    }
    wrap.style.setProperty('--away-possession', away);
    wrap.style.setProperty('--home-possession', home);
    ensureSoccerPossessionSegments(wrap);
    const awayValue = wrap.querySelector('[data-soccer-possession-value="away"]');
    const homeValue = wrap.querySelector('[data-soccer-possession-value="home"]');
    if (awayValue) awayValue.textContent = away;
    if (homeValue) homeValue.textContent = home;
  }

  function soccerPossessionTeamRow(card, side) {
    const row = document.createElement('div');
    const name = document.createElement('span');
    name.textContent = soccerTeamName(card, side);
    const value = document.createElement('strong');
    value.setAttribute('data-soccer-possession-value', side);
    row.append(name, value);
    return row;
  }

  function soccerPossessionBar() {
    const bar = document.createElement('div');
    bar.className = 'soccer-possession__bar';
    const away = document.createElement('span');
    away.setAttribute('data-soccer-possession-bar', 'away');
    const neutral = document.createElement('span');
    neutral.setAttribute('data-soccer-possession-bar', 'neutral');
    const home = document.createElement('span');
    home.setAttribute('data-soccer-possession-bar', 'home');
    bar.append(away, neutral, home);
    return bar;
  }

  function ensureSoccerPossessionSegments(wrap) {
    const bar = wrap.querySelector('.soccer-possession__bar');
    if (!bar || bar.querySelector('[data-soccer-possession-bar="neutral"]')) return;
    const neutral = document.createElement('span');
    neutral.setAttribute('data-soccer-possession-bar', 'neutral');
    const home = bar.querySelector('[data-soccer-possession-bar="home"]');
    if (home) bar.insertBefore(neutral, home);
    else bar.append(neutral);
  }

  function soccerTeamName(card, side) {
    const liveName = card.querySelector(`.live-team--${side} .live-name`);
    if (liveName) return Array.from(liveName.childNodes).find((node) => node.nodeType === Node.TEXT_NODE && node.textContent.trim())?.textContent.trim() || side;
    const worldCupName = card.querySelector(`[data-world-cup-side="${side}"] .world-cup-team-name`);
    if (worldCupName && worldCupName.textContent.trim()) return worldCupName.textContent.trim();
    return side === 'away' ? 'Away' : 'Home';
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
    lineupCache.set(game.ID, lineupCacheEntry({ Available: true, Lineup: game.Lineup }));
    markLineupPosted(game.ID);
  }

  function markLineupPosted(gameID) {
    document.querySelectorAll(`[data-lineup-game="${gameID}"]`).forEach((button) => {
      button.classList.add('lineup-button--ready');
      button.textContent = 'View Lineup';
    });
  }

  function hasCompleteLineup(lineup) {
    return Boolean(lineup && Array.isArray(lineup.Away) && lineup.Away.length && Array.isArray(lineup.Home) && lineup.Home.length);
  }

  function lineupCacheEntry(payload) {
    const complete = Boolean(payload && payload.Available && hasCompleteLineup(payload.Lineup));
    return {
      payload,
      expiresAt: complete ? Infinity : Date.now() + LINEUP_PARTIAL_TTL,
    };
  }

  function getCachedLineup(gameID) {
    const cached = lineupCache.get(gameID);
    if (!cached) return null;
    if (Date.now() > cached.expiresAt) {
      lineupCache.delete(gameID);
      return null;
    }
    return cached.payload;
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
      const stat = document.createElement('span');
      stat.className = 'lineup-stat';
      stat.textContent = entry.Position === 'P' ? '' : (entry.BattingAverage || '');
      const position = document.createElement('span');
      position.className = 'lineup-position';
      position.textContent = entry.Position || '';
      item.append(order, name, stat, position);
      list.append(item);
    });
    return [list];
  }

  function lineupSport(lineup, team) {
    return (team && team.Sport) || (lineup && lineup.AwayTeam && lineup.AwayTeam.Sport) || (lineup && lineup.HomeTeam && lineup.HomeTeam.Sport) || '';
  }

  function isSoccerLineup(lineup, team) {
    return ['MLS', 'FIFA'].includes(lineupSport(lineup, team));
  }

  function isBaseballLineup(lineup, team) {
    return lineupSport(lineup, team) === 'MLB';
  }

  function playerToken(entry, className = '') {
    const token = document.createElement('span');
    token.className = `lineup-field-player${className ? ` ${className}` : ''}`;
    token.title = [entry.Position, entry.Name].filter(Boolean).join(' - ');

    const pos = document.createElement('small');
    pos.textContent = entry.Position || '';
    const name = document.createElement('strong');
    name.textContent = entry.Name || 'TBD';
    token.append(pos, name);
    return token;
  }

  function soccerLineKey(position) {
    const pos = String(position || '').toUpperCase();
    if (['G', 'GK'].includes(pos)) return 'gk';
    if (['M', 'CM', 'AM', 'DM', 'CDM', 'CAM', 'LM', 'RM', 'LW', 'RW', 'W'].includes(pos)) return 'mid';
    if (['D', 'DEF', 'CB', 'LCB', 'RCB', 'CD', 'CD-L', 'CD-R', 'LB', 'RB', 'LWB', 'RWB'].includes(pos)) return 'def';
    if (pos.endsWith('B') || pos.startsWith('CD') || pos.includes('BACK')) return 'def';
    if (pos.includes('M') || pos === 'W') return 'mid';
    if (pos.includes('F') || pos.includes('ST') || pos.includes('ATT')) return 'fwd';
    return 'mid';
  }

  function renderSoccerField(entries) {
    if (!Array.isArray(entries) || entries.length === 0) return null;
    const field = document.createElement('div');
    field.className = 'lineup-field lineup-field--soccer';
    const pitch = document.createElement('div');
    pitch.className = 'lineup-soccer-pitch';

    const groups = { fwd: [], mid: [], def: [], gk: [] };
    entries.slice(0, 11).forEach((entry) => groups[soccerLineKey(entry.Position)].push(entry));
    [
      ['fwd', 'Attack'],
      ['mid', 'Midfield'],
      ['def', 'Defense'],
      ['gk', 'Keeper'],
    ].forEach(([key, label]) => {
      if (!groups[key].length) return;
      const line = document.createElement('div');
      line.className = `lineup-soccer-line lineup-soccer-line--${key}`;
      if (groups[key].length > 4) line.classList.add('lineup-soccer-line--dense');
      line.style.setProperty('--lineup-line-count', groups[key].length);
      line.setAttribute('aria-label', label);
      groups[key].forEach((entry) => line.append(playerToken(entry)));
      pitch.append(line);
    });
    field.append(pitch);
    return field;
  }

  function baseballPositionKey(position) {
    const pos = String(position || '').toUpperCase();
    if (pos.includes('P')) return 'p';
    if (pos === 'C') return 'c';
    if (pos === '1B') return 'b1';
    if (pos === '2B') return 'b2';
    if (pos === '3B') return 'b3';
    if (pos === 'SS') return 'ss';
    if (pos === 'LF') return 'lf';
    if (pos === 'CF') return 'cf';
    if (pos === 'RF') return 'rf';
    if (pos === 'DH') return 'dh';
    return '';
  }

  function renderBaseballField(entries) {
    if (!Array.isArray(entries) || entries.length === 0) return null;
    const field = document.createElement('div');
    field.className = 'lineup-field lineup-field--baseball';
    const diamond = document.createElement('div');
    diamond.className = 'lineup-baseball-diamond';

    entries.forEach((entry) => {
      const key = baseballPositionKey(entry.Position);
      if (!key) return;
      diamond.append(playerToken(entry, `lineup-field-player--${key}`));
    });
    if (!diamond.children.length) return null;
    field.append(diamond);
    return field;
  }

  function renderLineupField(lineup, team, entries) {
    if (isSoccerLineup(lineup, team)) return renderSoccerField(entries);
    if (isBaseballLineup(lineup, team)) return renderBaseballField(entries);
    return null;
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
    if (pitcher && pitcher.ERA) {
      const stat = document.createElement('span');
      stat.className = 'lineup-pitcher-stat';
      stat.textContent = `ERA ${pitcher.ERA}`;
      card.append(stat);
    }
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
      const field = renderLineupField(lineup, team, entries);
      if (field) section.append(field);
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
      const cached = getCachedLineup(gameID);
      if (cached) {
        renderLineup(modal, cached);
        return;
      }

      fetch(`/api/games/${encodeURIComponent(gameID)}/lineup`)
        .then((response) => response.ok ? response.json() : Promise.reject(response))
        .then((payload) => {
          lineupCache.set(gameID, lineupCacheEntry(payload));
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

  function initWorldCupBracketModal() {
    const modal = document.querySelector('[data-world-cup-bracket-modal]');
    const openButtons = document.querySelectorAll('[data-world-cup-bracket-open]');
    if (!modal || !openButtons.length) return;

    const close = () => {
      modal.hidden = true;
      document.body.classList.remove('world-cup-bracket-modal-open');
    };
    const open = () => {
      modal.hidden = false;
      document.body.classList.add('world-cup-bracket-modal-open');
      const scroller = modal.querySelector('.world-cup-bracket-modal__scroller');
      if (scroller) {
        requestAnimationFrame(() => {
          scroller.scrollLeft = Math.max(0, (scroller.scrollWidth - scroller.clientWidth) / 2);
        });
      }
    };

    openButtons.forEach((button) => button.addEventListener('click', open));
    modal.addEventListener('click', (event) => {
      if (event.target.closest('[data-world-cup-bracket-close]')) close();
    });
    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape' && !modal.hidden) close();
    });
  }

  function initWorldCupTabs() {
    const root = document.querySelector('[data-world-cup-tabs]');
    if (!root) return;

    const tabs = Array.from(root.querySelectorAll('[data-world-cup-tab]'));
    const panels = Array.from(root.querySelectorAll('[data-world-cup-panel]'));
    const activate = (name) => {
      tabs.forEach((tab) => {
        const active = tab.dataset.worldCupTab === name;
        tab.classList.toggle('active', active);
        tab.setAttribute('aria-selected', active ? 'true' : 'false');
      });
      panels.forEach((panel) => {
        const active = panel.dataset.worldCupPanel === name;
        panel.hidden = !active;
        panel.classList.toggle('active', active);
      });
    };

    tabs.forEach((tab) => {
      tab.addEventListener('click', () => activate(tab.dataset.worldCupTab));
    });
  }

  function worldCupMatchesFromPayload(cup) {
    const matches = [];
    const add = (items) => {
      if (Array.isArray(items)) matches.push(...items);
    };
    add(cup && cup.Live);
    add(cup && cup.Recent);
    add(cup && cup.Upcoming);
    if (cup && Array.isArray(cup.Bracket)) {
      cup.Bracket.forEach((round) => add(round && round.Matches));
    }
    return matches.filter((match) => match && match.ID);
  }

  function worldCupStatusLabel(match) {
    if (!match) return '';
    if (match.Status === 'Live') return match.Period || 'Live';
    if (match.Status === 'Final') return 'Final';
    return '';
  }

  function updateWorldCupMatchDOM(match) {
    if (match.Soccer && match.Soccer.Lineup) {
      lineupCache.set(match.ID, lineupCacheEntry({ Available: true, Lineup: match.Soccer.Lineup }));
      markLineupPosted(match.ID);
    }
    document.querySelectorAll(`[data-world-cup-match="${CSS.escape(match.ID)}"]`).forEach((card) => {
      const isLive = match.Status === 'Live';
      const isFinal = match.Status === 'Final';
      card.classList.toggle('world-cup-match--live', isLive);
      card.classList.toggle('world-cup-bracket-match--live', isLive);
      card.classList.toggle('world-cup-bracket-match--final', isFinal);

      const status = card.querySelector('[data-world-cup-status]');
      const label = worldCupStatusLabel(match);
      if (status && label) status.textContent = label;

      const awayScore = card.querySelector('[data-world-cup-score="away"]');
      const homeScore = card.querySelector('[data-world-cup-score="home"]');
      [awayScore, homeScore].forEach((score) => {
        if (score) score.hidden = !(isLive || isFinal);
      });
      if (awayScore) awayScore.textContent = match.AwayScore;
      if (homeScore) homeScore.textContent = match.HomeScore;

      const away = card.querySelector('[data-world-cup-side="away"]');
      const home = card.querySelector('[data-world-cup-side="home"]');
      if (away && home) {
        away.classList.remove('world-cup-bracket-team--winner');
        home.classList.remove('world-cup-bracket-team--winner');
        if (isFinal && match.AwayScore !== match.HomeScore) {
          (match.AwayScore > match.HomeScore ? away : home).classList.add('world-cup-bracket-team--winner');
        }
      }
      updateSoccerPulse(card, match.Soccer);
    });
  }

  function hasWorldCupActiveWindow(matches) {
    const now = Date.now();
    return matches.some((match) => {
      if (!match) return false;
      if (match.Status === 'Live') return true;
      if (match.Status !== 'Scheduled' || !match.StartTime) return false;
      const start = Date.parse(match.StartTime);
      if (!Number.isFinite(start)) return false;
      return now > start - (30 * 60 * 1000) && now < start + (3 * 60 * 60 * 1000);
    });
  }

  function pollWorldCup() {
    if (!document.querySelector('[data-world-cup-match]')) return Promise.resolve(WORLD_CUP_IDLE_POLL_INTERVAL);
    return fetch('/api/world-cup')
      .then((response) => response.ok ? response.json() : Promise.reject(response))
      .then((cup) => {
        const matches = worldCupMatchesFromPayload(cup);
        matches.forEach(updateWorldCupMatchDOM);
        return hasWorldCupActiveWindow(matches) ? WORLD_CUP_ACTIVE_POLL_INTERVAL : WORLD_CUP_IDLE_POLL_INTERVAL;
      })
      .catch(() => WORLD_CUP_IDLE_POLL_INTERVAL);
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
    return fetch('/api/scores')
      .then((response) => response.json())
      .then((games) => {
        games.forEach((game) => {
          updateCardDOM(game);
          updateScheduleDOM(game);
          emit('score_update', game);
        });
        return games.some((game) => game && game.Status === 'Live') ? SCORE_LIVE_POLL_INTERVAL : SCORE_IDLE_POLL_INTERVAL;
      })
      .catch(() => SCORE_IDLE_POLL_INTERVAL);
  }

  function schedulePoll(pollFn, fallbackInterval) {
    pollFn()
      .then((interval) => {
        setTimeout(() => schedulePoll(pollFn, fallbackInterval), interval || fallbackInterval);
      })
      .catch(() => {
        setTimeout(() => schedulePoll(pollFn, fallbackInterval), fallbackInterval);
      });
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
  initWorldCupBracketModal();
  initWorldCupTabs();
  schedulePoll(pollWorldCup, WORLD_CUP_IDLE_POLL_INTERVAL);
  schedulePoll(pollScores, SCORE_IDLE_POLL_INTERVAL);

  window.PhillyGametime = {
    on(eventName, callback) {
      document.addEventListener(`phillyGametime:${eventName}`, (event) => callback(event.detail));
    },
  };
})();
