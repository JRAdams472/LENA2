/// The client-side agent loop — a Dart port of the web client's
/// lib/ai/agent.ts. Mirrors the server's Ask: model emits JSON tool calls,
/// each executes server-side through callAssistantTool (household-scoped,
/// read-only), results feed back, and the loop ends on a final answer or
/// the round cap.
library;

import 'dart:convert';

import 'engine.dart';
import 'protocol.dart';

class AgentException implements Exception {
  AgentException(this.message);
  final String message;
  @override
  String toString() => 'AgentException: $message';
}

class AgentRunResult {
  const AgentRunResult({required this.answer, required this.tools});
  final String answer;
  final List<String> tools;
}

/// [callTool] runs one read-only assistant tool server-side and returns
/// the JSON result string.
Future<AgentRunResult> runAgent({
  required LocalEngine engine,
  required String systemPrompt,
  required List<AgentToolSpec> tools,
  required Future<String> Function(String name, String argsJson) callTool,
  required String question,
  int maxRounds = 5,
}) async {
  final messages = <EngineMessage>[
    EngineMessage(EngineRole.system, buildSystemPrompt(systemPrompt, tools)),
    EngineMessage(EngineRole.user, question),
  ];
  final usedTools = <String>[];

  for (var round = 0; round <= maxRounds; round++) {
    final raw = await engine.chat(messages);
    messages.add(EngineMessage(EngineRole.assistant, raw));
    final parsed = parseModelReply(raw, tools);

    if (parsed is AnswerReply) {
      return AgentRunResult(answer: parsed.text, tools: usedTools);
    }
    if (parsed is MalformedReply) {
      messages.add(const EngineMessage(EngineRole.user, retryNudge));
      continue;
    }

    final results = <({String name, String result})>[];
    for (final call in (parsed as ToolsReply).calls) {
      usedTools.add(call.name);
      String result;
      try {
        result = await callTool(call.name, jsonEncode(call.arguments));
      } catch (err) {
        // Network/server failures are reported to the model the same way
        // the server loop reports handler errors — as data, not a crash.
        result = jsonEncode({'error': err.toString()});
      }
      results.add((name: call.name, result: result));
    }
    messages.add(EngineMessage(EngineRole.user, formatToolResults(results)));
  }
  throw AgentException('assistant exceeded $maxRounds tool rounds');
}
