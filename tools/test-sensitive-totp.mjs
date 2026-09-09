import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import assert from 'node:assert/strict';
import test from 'node:test';

const template = readFileSync(new URL('../web/templates/base.html', import.meta.url), 'utf8');
const submitStart = template.indexOf("  const sensitivePending = new WeakSet();");
const submitEnd = template.indexOf('  }, true);', submitStart) + '  }, true);'.length;

function fixture({ fresh = false, stale = false, code = async () => '123456' } = {}) {
  let listener;
  const sent = [];
  const inputs = [];
  const element = () => ({ type: 'hidden', name: 'totp_code', value: '', remove() {
    const index = inputs.indexOf(this);
    if (index >= 0) inputs.splice(index, 1);
  } });
  if (stale) inputs.push(Object.assign(element(), { value: '654321' }));
  class Form {
    method = 'post';
    hasAttribute() { return false; }
    getAttribute() { return '/settings/maintenance/nas'; }
    querySelectorAll() { return inputs.filter(input => input.type === 'hidden'); }
    querySelector() { return inputs[0]; }
    appendChild(input) { inputs.push(input); }
    requestSubmit(submitter) {
      const event = makeEvent(submitter);
      listener(event);
      if (!event.defaultPrevented) sent.push({ action: submitter.value, code: inputs[0]?.value });
    }
  }
  const form = new Form();
  const makeEvent = submitter => ({ target: form, submitter, defaultPrevented: false,
    preventDefault() { this.defaultPrevented = true; } });
  const context = {
    HTMLFormElement: Form,
    document: { addEventListener: (_, callback) => listener = callback, createElement: element },
    sensitiveBypass: new WeakSet(),
    sensitiveTOTPValidUntil: fresh ? Date.now() + 600000 : 0,
    releaseModalFocusForDialog: async () => () => {},
    pbkDialog: { code },
    window: { location: { pathname: '/settings/maintenance' } },
  };
  vm.runInNewContext(template.slice(submitStart, submitEnd), context);
  return { sent, inputs, submit: action => listener(makeEvent({ name: 'action', value: action })) };
}

for (const action of ['test', 'save']) {
  test(`${action}: use a new code and preserve submitter, then remove code`, async () => {
    const f = fixture({ stale: true });
    await f.submit(action);
    assert.deepEqual(f.sent, [{ action, code: '123456' }]);
    assert.equal(f.inputs.length, 0);
    await f.submit(action);
    assert.equal(f.sent.length, 2);
    assert.equal(f.inputs.length, 0);
  });
}

test('fresh session must not submit an expired hidden code', async () => {
  const f = fixture({ fresh: true, stale: true, code: () => { throw Error('unexpected prompt'); } });
  await f.submit('test');
  assert.equal(f.inputs.length, 0);
});

test('double submission opens one prompt and submits once', async () => {
  let finish;
  const f = fixture({ code: () => new Promise(resolve => finish = resolve) });
  const first = f.submit('test');
  await Promise.resolve();
  await f.submit('test');
  finish('123456');
  await first;
  assert.equal(f.sent.length, 1);
});

test('Enter consumes its default action and repeated key events cannot confirm again', () => {
  const start = template.indexOf("    input.addEventListener('keydown', e => {");
  const end = template.indexOf('    });', start) + '    });'.length;
  let listener;
  const values = [];
  vm.runInNewContext(template.slice(start, end), {
    input: { value: '012345', addEventListener: (_, callback) => listener = callback },
    finish: value => values.push(value),
  });
  for (const repeat of [false, true]) {
    let prevented = false;
    let stopped = false;
    listener({ key: 'Enter', repeat, preventDefault() { prevented = true; }, stopPropagation() { stopped = true; } });
    assert.equal(prevented, true);
    assert.equal(stopped, true);
  }
  assert.deepEqual(values, ['012345']);
});
