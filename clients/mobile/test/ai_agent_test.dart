import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/ai/agent.dart';
import 'package:lena_mobile/ai/engine.dart';

const _tool = AgentToolSpec(
  name: 'get_pantry_inventory',
  description: 'Read pantry items',
  parametersJson: '{"type":"object","properties":{}}',
);

/// Scripted replies in order; records every request (copied — the agent
/// mutates the message list between calls).
class _FakeEngine implements LocalEngine {
  _FakeEngine(this.replies);
  final List<String> replies;
  final requests = <List<EngineMessage>>[];
  var disposed = false;

  @override
  String get id => 'fake';
  @override
  String get label => 'fake';
  @override
  Future<String> chat(List<EngineMessage> messages) async {
    requests.add(List.of(messages));
    return replies.isEmpty ? '{"answer":"done"}' : replies.removeAt(0);
  }

  @override
  Future<void> dispose() async {
    disposed = true;
  }
}

void main() {
  test('returns the final answer for a plain reply', () async {
    final out = await runAgent(
      engine: _FakeEngine(['{"answer":"milk is fine"}']),
      systemPrompt: 'You are Dot.',
      tools: const [_tool],
      callTool: (_, __) async => '{}',
      question: 'is milk ok?',
    );
    expect(out.answer, 'milk is fine');
    expect(out.tools, isEmpty);
  });

  test('dispatches a tool call and feeds the result back', () async {
    final engine = _FakeEngine([
      '{"toolCalls":[{"name":"get_pantry_inventory","arguments":{}}]}',
      '{"answer":"2 items"}',
    ]);
    final calls = <String>[];
    final out = await runAgent(
      engine: engine,
      systemPrompt: 'You are Dot.',
      tools: const [_tool],
      callTool: (name, args) async {
        calls.add(name);
        return '{"count":2}';
      },
      question: 'q',
    );
    expect(calls, ['get_pantry_inventory']);
    expect(out.tools, ['get_pantry_inventory']);
    expect(out.answer, '2 items');
    // The second model request carries the tool results as a user turn.
    final second = engine.requests[1];
    expect(second.last.role, EngineRole.user);
    expect(second.last.content, contains('"count":2'));
  });

  test('nudges the model once after malformed protocol output', () async {
    final engine = _FakeEngine([
      '{"toolCalls":[{"name":"bogus_tool","arguments":{}}]}',
      '{"answer":"recovered"}',
    ]);
    final out = await runAgent(
      engine: engine,
      systemPrompt: 'You are Dot.',
      tools: const [_tool],
      callTool: (_, __) async => '{}',
      question: 'q',
    );
    expect(out.answer, 'recovered');
    expect(engine.requests[1].last.role, EngineRole.user);
    expect(engine.requests[1].last.content, contains('Reply with ONLY'));
  });

  test('reports tool transport failures to the model, not a crash', () async {
    final engine = _FakeEngine([
      '{"toolCalls":[{"name":"get_pantry_inventory","arguments":{}}]}',
      '{"answer":"could not check"}',
    ]);
    final out = await runAgent(
      engine: engine,
      systemPrompt: 'You are Dot.',
      tools: const [_tool],
      callTool: (_, __) async => throw Exception('network down'),
      question: 'q',
    );
    expect(out.answer, 'could not check');
    expect(engine.requests[1].last.content, contains('network down'));
  });

  test('throws after exceeding the tool-round cap', () async {
    final engine = _FakeEngine(List.generate(
        10,
        (_) =>
            '{"toolCalls":[{"name":"get_pantry_inventory","arguments":{}}]}'));
    expect(
      runAgent(
        engine: engine,
        systemPrompt: 'p',
        tools: const [_tool],
        callTool: (_, __) async => '{}',
        question: 'q',
        maxRounds: 2,
      ),
      throwsA(isA<AgentException>()),
    );
  });
}
