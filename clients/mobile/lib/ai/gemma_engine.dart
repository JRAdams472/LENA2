/// flutter_gemma-backed [LocalEngine]. One inference session is kept for
/// the agent's whole run; the runtime retains history internally, so each
/// chat() call only appends the user turns the agent added since the last
/// call.
library;

import 'engine.dart';
import 'gemma_binding.dart';

class GemmaEngine implements LocalEngine {
  GemmaEngine(this._binding);

  final GemmaBinding _binding;
  GemmaSessionHandle? _session;
  int _sent = 0;

  @override
  String get id => 'gemma';

  @override
  String get label => 'On this device (Gemma)';

  @override
  Future<String> chat(List<EngineMessage> messages) async {
    if (_session == null) {
      final system = messages
          .where((m) => m.role == EngineRole.system)
          .map((m) => m.content)
          .join('\n');
      _session = await _binding.createSession(
        systemInstruction: system.isEmpty ? null : system,
      );
      _sent = 0;
    }
    // Skip system messages: the first one was folded into the session's
    // system instruction. Feed only new user turns; assistant replies in
    // [messages] were produced by this very session, so re-feeding them
    // would double-count context.
    for (final m in messages.skip(_sent)) {
      if (m.role == EngineRole.user) {
        await _session!.addUserMessage(m.content);
      }
    }
    _sent = messages.length;
    return _session!.reply();
  }

  @override
  Future<void> dispose() async {
    final s = _session;
    _session = null;
    _sent = 0;
    await s?.close();
  }
}
