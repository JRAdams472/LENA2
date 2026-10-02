/// Shared types for the local-inference assistant path. A [LocalEngine] is
/// any on-device chat runtime (flutter_gemma today); the agent loop in
/// agent.dart drives engines through a shared JSON tool protocol because
/// 1-3B models lack reliable native function-calling.
library;

/// One assistant tool spec as served by the assistantTools query.
class AgentToolSpec {
  const AgentToolSpec({
    required this.name,
    required this.description,
    required this.parametersJson,
  });
  final String name;
  final String description;
  final String parametersJson;
}

enum EngineRole { system, user, assistant }

class EngineMessage {
  const EngineMessage(this.role, this.content);
  final EngineRole role;
  final String content;
}

/// One tool invocation the model asked for.
class ToolCallRequest {
  const ToolCallRequest(this.name, this.arguments);
  final String name;
  final Map<String, dynamic> arguments;
}

abstract class LocalEngine {
  /// Stable identifier for badges.
  String get id;

  /// Human label, e.g. "On this device (Gemma)".
  String get label;

  /// One completion over the whole conversation so far. Returns the raw
  /// model text — parsing is the protocol layer's job.
  Future<String> chat(List<EngineMessage> messages);

  /// Release model resources (session/context) when the engine is dropped.
  Future<void> dispose();
}

/// The outcome of one assistant turn — same shape regardless of where
/// inference ran.
class AssistantResult {
  const AssistantResult({
    required this.answer,
    required this.tools,
    required this.engineLabel,
    required this.isLocal,
  });
  final String answer;
  final List<String> tools;
  final String engineLabel;
  final bool isLocal;
}
