import type {ReactNode} from 'react';
import {useState} from 'react';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import {ArrowRight, Search} from 'lucide-react';

import {COMMAND_COUNT, FEATURE_GROUPS} from '../data/featureMap';
import styles from './features.module.css';

export default function Features(): ReactNode {
  const [q, setQ] = useState('');
  const [group, setGroup] = useState('all');
  const needle = q.trim().toLowerCase();

  const groups = FEATURE_GROUPS.map((g) => {
    const items = needle
      ? g.items.filter((it) => `${it.cmd} ${it.what} ${it.kw} ${g.title} ${g.when}`.toLowerCase().includes(needle))
      : g.items;
    const visible = (group === 'all' || group === g.id) && items.length > 0;
    return {...g, items, visible};
  });
  const count = groups.reduce((n, g) => n + (g.visible ? g.items.length : 0), 0);
  const chips = [{id: 'all', title: 'All'}, ...FEATURE_GROUPS];

  return (
    <Layout title="Feature map" description="Every skillshare command, grouped by the job you're trying to do.">
      <div className={styles.page}>
        <header className={`container ${styles.header}`}>
          <div className={styles.headRow}>
            <div className={styles.headText}>
              <Heading as="h1" className={styles.title}>
                Find the feature <span className={styles.mark}>you actually need.</span>
              </Heading>
              <p className={styles.lead}>
                {COMMAND_COUNT} commands and 100+ docs pages, grouped by the job you're trying to do. Type a task, a command,
                or a word you remember from the error.
              </p>
            </div>
            <span className={styles.hint}>start from the job,<br />not the command list</span>
          </div>

          <div className={styles.searchWrap}>
            <label htmlFor="feature-q" className={styles.sr}>Filter commands by task, name or keyword</label>
            <Search className={styles.searchIcon} size={24} aria-hidden="true" />
            <input
              id="feature-q"
              className={styles.search}
              type="search"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Try “share with my team”, “audit”, “windows”, “symlink”…"
              autoComplete="off"
            />
          </div>

          <div className={styles.chipRow}>
            <div className={styles.chips}>
              {chips.map((c) => (
                <button
                  key={c.id}
                  type="button"
                  className={`${styles.chip} ${group === c.id ? styles.chipOn : ''}`}
                  onClick={() => setGroup(c.id)}
                  aria-pressed={group === c.id}
                >
                  {c.title}
                </button>
              ))}
            </div>
            <span className={styles.count}>
              {count} of {COMMAND_COUNT} commands
            </span>
          </div>
        </header>

        <main className={`container ${styles.grid}`}>
          {groups.filter((g) => g.visible).map((g) => (
            <section key={g.id} className={styles.group}>
              <div className={styles.groupHead}>
                <span className={styles.num}>{g.num}</span>
                <Heading as="h2" className={styles.groupTitle}>{g.title}</Heading>
              </div>
              <p className={styles.when}>{g.when}</p>
              <div className={styles.rows}>
                {g.items.map((it) => (
                  <Link key={it.cmd} to={it.href} className={styles.row}>
                    <span><code className={styles.cmd}>{it.cmd}</code></span>
                    <span className={styles.what}>{it.what}</span>
                    <ArrowRight className={styles.arrow} size={18} aria-hidden="true" />
                  </Link>
                ))}
              </div>
              <div className={styles.guides}>
                <span className={styles.guidesLabel}>Guides</span>
                {g.guides.map((d) => (
                  <Link key={d.href} to={d.href} className={styles.guide}>{d.label}</Link>
                ))}
              </div>
            </section>
          ))}

          {count === 0 && (
            <div className={styles.empty}>
              <p className={styles.emptyTitle}>Nothing matches “{q}”.</p>
              <p className={styles.lead}>Try a shorter word, or browse the full reference below.</p>
              <button type="button" className="button button--secondary" onClick={() => { setQ(''); setGroup('all'); }}>
                Clear filter
              </button>
            </div>
          )}
        </main>

        <section className="container">
          <div className={styles.fallback}>
            <div>
              <Heading as="h2" className={styles.fallbackTitle}>Still can't find it?</Heading>
              <p className={styles.lead}>Everything is indexed in the reference, and the FAQ covers the questions people actually ask.</p>
            </div>
            <div className={styles.fallbackLinks}>
              <Link className="button button--primary" to="/docs/reference/commands">Full command reference</Link>
              <Link className="button button--secondary" to="/docs/troubleshooting/faq">FAQ</Link>
              <Link className="button button--secondary" to="/docs/troubleshooting">Troubleshooting</Link>
              <Link className="button button--secondary" href="https://github.com/runkids/skillshare/issues">Ask on GitHub</Link>
            </div>
          </div>
        </section>
      </div>
    </Layout>
  );
}
