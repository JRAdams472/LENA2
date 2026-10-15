# flutter_gemma's MediaPipe native lib references generated protos that are
# not packaged — R8 fails the release build without these dontwarns.
-dontwarn com.google.mediapipe.proto.CalculatorProfileProto$CalculatorProfile
-dontwarn com.google.mediapipe.proto.GraphTemplateProto$CalculatorGraphTemplate
