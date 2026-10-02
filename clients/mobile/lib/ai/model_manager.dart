/// On-device model lifecycle: install (with progress + cancellation),
/// installed check, and deletion. The download source is configurable —
/// LENA_LOCAL_MODEL_URL lets a build point at a self-hosted artifact when
/// the default Hugging Face mirror is gated or unreachable.
library;

import 'package:flutter_gemma/flutter_gemma.dart' show ModelFileType, ModelType;

import 'gemma_binding.dart';

/// Default model artifact: Gemma 3 1B instruction-tuned, int4, LiteRT-LM
/// format (~550 MB). litert-community repos are license-gated — pass
/// LENA_LOCAL_MODEL_TOKEN at build time, or host an ungated mirror via
/// LENA_LOCAL_MODEL_URL.
const String localModelUrl = String.fromEnvironment(
  'LENA_LOCAL_MODEL_URL',
  defaultValue:
      'https://huggingface.co/litert-community/Gemma3-1B-IT/resolve/main/gemma3-1b-it-int4.litertlm',
);
const String localModelToken = String.fromEnvironment(
  'LENA_LOCAL_MODEL_TOKEN',
);
const String localModelId = String.fromEnvironment(
  'LENA_LOCAL_MODEL_ID',
  defaultValue: 'gemma3-1b-it-int4',
);

enum ModelInstallState { idle, downloading, installed, failed }

/// Coordinates model install/remove through the injected binding. The
/// controller owns the instance; the screen only reads state.
class LocalModelManager {
  LocalModelManager(this._binding);

  final GemmaBinding _binding;
  GemmaCancelHandle? _cancel;

  Future<bool> isInstalled() => _binding.isModelInstalled(localModelId);

  /// Downloads the configured model. Throws on failure; cancellation throws
  /// a flutter_gemma cancel error the caller maps back to idle.
  Future<void> install({void Function(int progress)? onProgress}) {
    final cancel = _binding.newCancelHandle();
    _cancel = cancel;
    return _binding
        .installModel(
          url: localModelUrl,
          modelType: ModelType.gemmaIt,
          fileType: ModelFileType.litertlm,
          token: localModelToken.isEmpty ? null : localModelToken,
          onProgress: onProgress,
          cancelHandle: cancel,
        )
        .whenComplete(() => _cancel = null);
  }

  void cancelInstall() => _cancel?.cancel('User cancelled');

  Future<void> uninstall() => _binding.uninstallModel(localModelId);
}
