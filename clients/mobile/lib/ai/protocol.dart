/// The shared JSON tool protocol for small local models — a Dart port of
/// the web client's lib/ai/protocol.ts. Native tool calling is unreliable
/// or absent on 1-3B runtimes, so the agent instead requires every model
/// reply to be one JSON object:
///
///   {"toolCalls":[{"name":"get_pantry_inventory","arguments":{...}}]}
///   {"answer":"You have 2 gallons of milk."}
///
/// Parsing is deliberately forgiving — models wrap JSON in markdown fences
/// or prose — but tool names are checked against the served spec list, so
/// hallucinated calls turn into a malformed reply the agent can retry.
library;

import 'dart:convert';

import 'engine.dart';

sealed class ParsedReply {
  const ParsedReply();
}

class ToolsReply extends ParsedReply {
  const ToolsReply(this.calls);
  final List<ToolCallRequest> calls;
}

class AnswerReply extends ParsedReply {
  const AnswerReply(this.text);
  final String text;
}

class MalformedReply extends ParsedReply {
  const MalformedReply();
}

/// Appended as a user turn when the model replies with something that
/// parsed as a protocol object but was unusable.
const String retryNudge =
    'Reply with ONLY one JSON object: {"toolCalls":[...]} to call tools, or {"answer":"..."} for the final answer.';

/// Mirrors the server's tool argument cap so a runaway model can't force
/// oversized payloads over the wire.
const int maxCallArgsBytes = 4096;

/// Appends the protocol contract and the served tool catalog to the
/// server-owned system prompt.
String buildSystemPrompt(String basePrompt, List<AgentToolSpec> tools) {
  final catalog = tools.map((t) {
    var params = t.parametersJson;
    try {
      params = jsonEncode(jsonDecode(t.parametersJson));
    } catch (_) {
      /* serve verbatim */
    }
    return '- ${t.name}: ${t.description}\n  args schema: $params';
  }).join('\n');
  return '$basePrompt\n'
      '\n'
      'TOOL PROTOCOL: tools are read-only lookups; calling one fetches real household data.\n'
      'To call tools, reply with EXACTLY one JSON object and nothing else:\n'
      '{"toolCalls":[{"name":"<tool name>","arguments":{<args matching the schema>}}]}\n'
      'To give the final answer, reply with EXACTLY one JSON object and nothing else:\n'
      '{"answer":"<your reply>"}\n'
      'Never mix prose and JSON. Never invent tool names or results.\n'
      '\n'
      'Available tools:\n'
      '$catalog';
}

/// Removes markdown code fences a model may wrap output in.
String stripFences(String text) {
  final m = RegExp(r'```(?:json)?\s*([\s\S]*?)```').firstMatch(text);
  return m != null ? m.group(1)!.trim() : text.trim();
}

/// Finds a JSON object in possibly-noisy model output: the whole reply
/// first, then the widest brace span as a fallback.
Map<String, dynamic>? extractJsonObject(String raw) {
  final text = stripFences(raw);
  final first = text.indexOf('{');
  final last = text.lastIndexOf('}');
  final candidates = <String>[
    text,
    if (first >= 0 && last > first) text.substring(first, last + 1),
  ];
  for (final candidate in candidates) {
    if (!candidate.startsWith('{')) continue;
    try {
      final decoded = jsonDecode(candidate);
      if (decoded is Map<String, dynamic>) return decoded;
      if (decoded is Map) return decoded.cast<String, dynamic>();
    } catch (_) {
      /* try the next candidate */
    }
  }
  return null;
}

/// Normalizes the toolCalls array: each call needs a known name and
/// arguments that fit the wire cap (arguments may arrive as an object or a
/// stringified object — both occur in the wild).
List<ToolCallRequest>? _parseToolCalls(dynamic v, Set<String> known) {
  if (v is! List || v.isEmpty) return null;
  final calls = <ToolCallRequest>[];
  for (final item in v) {
    if (item is! Map) return null;
    final name = item['name'];
    final args = item['arguments'];
    if (name is! String || !known.contains(name)) return null;
    var parsed = <String, dynamic>{};
    if (args is String && args.trim().isNotEmpty) {
      try {
        final p = jsonDecode(args);
        if (p is! Map) return null;
        parsed = p.cast<String, dynamic>();
      } catch (_) {
        return null;
      }
    } else if (args != null) {
      if (args is! Map) return null;
      parsed = args.cast<String, dynamic>();
    }
    if (jsonEncode(parsed).length > maxCallArgsBytes) return null;
    calls.add(ToolCallRequest(name, parsed));
  }
  return calls;
}

/// Classifies raw model output for the agent loop.
ParsedReply parseModelReply(String raw, List<AgentToolSpec> tools) {
  final known = tools.map((t) => t.name).toSet();
  final obj = extractJsonObject(raw);

  if (obj != null) {
    if (obj.containsKey('toolCalls')) {
      final calls = _parseToolCalls(obj['toolCalls'], known);
      if (calls != null) return ToolsReply(calls);
      // A toolCalls key that didn't produce valid calls is a protocol
      // violation — retry beats silently answering.
      return const MalformedReply();
    }
    final answer = obj['answer'];
    if (answer is String && answer.trim().isNotEmpty) {
      return AnswerReply(answer.trim());
    }
    return const MalformedReply();
  }

  // No JSON object at all: the model answered plainly. Accept it rather
  // than erroring — a small model that forgets the protocol still helped.
  final text = stripFences(raw).trim();
  if (text.isNotEmpty) return AnswerReply(text);
  return const MalformedReply();
}

/// Serializes executed calls into the next user turn.
String formatToolResults(List<({String name, String result})> results) {
  final payload = results.map((r) {
    dynamic parsed = r.result;
    try {
      parsed = jsonDecode(r.result);
    } catch (_) {
      /* keep the raw string */
    }
    return {'name': r.name, 'result': parsed};
  }).toList();
  return 'Tool results:\n${jsonEncode(payload)}';
}
