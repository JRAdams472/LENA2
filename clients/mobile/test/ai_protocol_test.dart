import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/ai/engine.dart';
import 'package:lena_mobile/ai/protocol.dart';

const _tool = AgentToolSpec(
  name: 'get_pantry_inventory',
  description: 'Read pantry items',
  parametersJson: '{"type":"object","properties":{}}',
);

void main() {
  group('parseModelReply', () {
    test('parses a bare JSON tool call', () {
      final r = parseModelReply(
        '{"toolCalls":[{"name":"get_pantry_inventory","arguments":{}}]}',
        const [_tool],
      );
      expect(r, isA<ToolsReply>());
      expect((r as ToolsReply).calls.single.name, 'get_pantry_inventory');
    });

    test('parses a fenced JSON answer', () {
      final r = parseModelReply(
        '```json\n{"answer":"You have milk"}\n```',
        const [_tool],
      );
      expect(r, isA<AnswerReply>());
      expect((r as AnswerReply).text, 'You have milk');
    });

    test('accepts a plain-text reply as an answer', () {
      final r = parseModelReply('Try a stir fry tonight!', const [_tool]);
      expect(r, isA<AnswerReply>());
      expect((r as AnswerReply).text, 'Try a stir fry tonight!');
    });

    test('rejects calls to unknown tool names', () {
      final r = parseModelReply(
        '{"toolCalls":[{"name":"drop_database","arguments":{}}]}',
        const [_tool],
      );
      expect(r, isA<MalformedReply>());
    });

    test('rejects a toolCalls key with empty array', () {
      final r = parseModelReply('{"toolCalls":[]}', const [_tool]);
      expect(r, isA<MalformedReply>());
    });

    test('accepts stringified arguments objects', () {
      final r = parseModelReply(
        '{"toolCalls":[{"name":"get_pantry_inventory","arguments":"{\\"a\\":1}"}]}',
        const [_tool],
      );
      expect(r, isA<ToolsReply>());
      expect((r as ToolsReply).calls.single.arguments, {'a': 1});
    });

    test('rejects oversized arguments', () {
      final big = 'x' * (maxCallArgsBytes + 10);
      final r = parseModelReply(
        '{"toolCalls":[{"name":"get_pantry_inventory","arguments":{"q":"$big"}}]}',
        const [_tool],
      );
      expect(r, isA<MalformedReply>());
    });

    test('extracts JSON embedded in prose', () {
      final r = parseModelReply(
        'Sure! {"answer":"done"} hope that helps',
        const [_tool],
      );
      expect(r, isA<AnswerReply>());
      expect((r as AnswerReply).text, 'done');
    });
  });

  group('buildSystemPrompt', () {
    test('appends the protocol contract and tool catalog', () {
      final p = buildSystemPrompt('You are Dot.', const [_tool]);
      expect(p, contains('You are Dot.'));
      expect(p, contains('TOOL PROTOCOL'));
      expect(p, contains('get_pantry_inventory'));
      expect(p, contains('{"answer"'));
    });
  });

  group('formatToolResults', () {
    test('serializes results and parses JSON payloads', () {
      final out = formatToolResults([
        (name: 'get_pantry_inventory', result: '{"items":[]}'),
        (name: 'get_pantry_inventory', result: 'plain'),
      ]);
      expect(out, contains('Tool results:'));
      expect(out, contains('"items":[]'));
      expect(out, contains('"plain"'));
    });
  });
}
