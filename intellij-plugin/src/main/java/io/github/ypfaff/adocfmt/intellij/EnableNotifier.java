package io.github.ypfaff.adocfmt.intellij;

import com.intellij.ide.util.PropertiesComponent;
import com.intellij.notification.NotificationAction;
import com.intellij.notification.NotificationGroupManager;
import com.intellij.notification.NotificationType;
import com.intellij.openapi.fileEditor.FileEditorManager;
import com.intellij.openapi.fileEditor.FileEditorManagerListener;
import com.intellij.openapi.project.Project;
import com.intellij.openapi.vfs.VirtualFile;
import org.jetbrains.annotations.NotNull;

/** Offers to switch adocfmt on when the first AsciiDoc file opens in a project where it is off. */
final class EnableNotifier implements FileEditorManagerListener {
  // The project's PropertiesComponent lives in workspace.xml, which is never
  // committed, so each user is asked once.
  static final String OFFERED = "adocfmt.enableOffered";

  @Override
  public void fileOpened(@NotNull FileEditorManager source, @NotNull VirtualFile file) {
    Project project = source.getProject();
    PropertiesComponent workspace = PropertiesComponent.getInstance(project);
    if (!AdocfmtFormattingService.isAsciiDoc(file.getName())
        || AdocfmtSettings.getInstance(project).isEnabled()
        || workspace.isTrueValue(OFFERED)) {
      return;
    }
    // Marked on showing, so it is not offered again however it is closed.
    workspace.setValue(OFFERED, true);
    offer(project);
  }

  private static void offer(Project project) {
    NotificationGroupManager.getInstance()
        .getNotificationGroup("Enable adocfmt")
        .createNotification(
            "Enable adocfmt",
            "Format the AsciiDoc files in this project with adocfmt.",
            NotificationType.INFORMATION)
        .addAction(
            NotificationAction.createSimpleExpiring(
                "Enable for this project",
                () -> AdocfmtSettings.getInstance(project).setEnabled(true)))
        .notify(project);
  }
}
