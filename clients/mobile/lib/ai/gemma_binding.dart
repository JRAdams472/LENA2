/// The seam between the assistant layer and flutter_gemma. Everything
/// platform-channel-dependent lives behind [GemmaBinding] so tests (and any
/// future second runtime) can substitute a fake without touching
/// flutter_gemma's statics.
library;

import 'package:flutter_gemma/flutter_gemma.dart';

/// A live inference session. The underlying runtime keeps its own
/// conversation history, so callers only append user turns.
abstract class GemmaSessionHandle {
  Future<void> addUserMessage(String text);
  Future<String> reply();
  Future<void> close();
}

abstract class GemmaCancelHandle {
  void cancel([String? reason]);
}

abstract class GemmaBinding {
  /// A fresh cancellation handle for [installModel].
  GemmaCancelHandle newCancelHandle();

  /// Whether the model identified by [modelId] is already on-device.
  Future<bool> isModelInstalled(String modelId);

  /// Download and activate a model. [onProgress] receives 0-100.
  Future<void> installModel({
    required String url,
    ModelType modelType = ModelType.gemmaIt,
    ModelFileType fileType = ModelFileType.litertlm,
    String? token,
    void Function(int progress)? onProgress,
    GemmaCancelHandle? cancelHandle,
  });

  Future<void> uninstallModel(String modelId);

  /// Open an inference session against the active model.
  Future<GemmaSessionHandle> createSession({String? systemInstruction});
}

class _SessionHandle implements GemmaSessionHandle {
  _SessionHandle(this._session);
  final InferenceModelSession _session;

  @override
  Future<void> addUserMessage(String text) =>
      _session.addQueryChunk(Message.text(text: text, isUser: true));

  @override
  Future<String> reply() => _session.getResponse();

  @override
  Future<void> close() => _session.close();
}

class _CancelHandle implements GemmaCancelHandle {
  _CancelHandle(this.token);
  final CancelToken token;
  @override
  void cancel([String? reason]) => token.cancel(reason ?? 'cancelled');
}

/// The real flutter_gemma-backed binding used by the app.
class FlutterGemmaBinding implements GemmaBinding {
  @override
  GemmaCancelHandle newCancelHandle() => _CancelHandle(CancelToken());

  @override
  Future<bool> isModelInstalled(String modelId) =>
      FlutterGemma.isModelInstalled(modelId);

  @override
  Future<void> installModel({
    required String url,
    ModelType modelType = ModelType.gemmaIt,
    ModelFileType fileType = ModelFileType.litertlm,
    String? token,
    void Function(int progress)? onProgress,
    GemmaCancelHandle? cancelHandle,
  }) async {
    final builder = FlutterGemma.installModel(
      modelType: modelType,
      fileType: fileType,
    // foreground: null → auto (>500MB downloads get a foreground service).
    ).fromNetwork(url, token: token);
    if (onProgress != null) builder.withProgress(onProgress);
    if (cancelHandle is _CancelHandle) {
      builder.withCancelToken(cancelHandle.token);
    }
    await builder.install();
  }

  @override
  Future<void> uninstallModel(String modelId) =>
      FlutterGemma.uninstallModel(modelId);

  @override
  Future<GemmaSessionHandle> createSession({String? systemInstruction}) async {
    final model = await FlutterGemma.getActiveModel(maxTokens: 4096);
    final session = await model.createSession(
      systemInstruction: systemInstruction,
      temperature: 0.8,
      topK: 1,
    );
    return _SessionHandle(session);
  }
}
