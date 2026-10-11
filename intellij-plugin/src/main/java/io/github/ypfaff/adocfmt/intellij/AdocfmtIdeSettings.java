package io.github.ypfaff.adocfmt.intellij;

import com.intellij.openapi.application.ApplicationManager;
import com.intellij.openapi.components.PersistentStateComponent;
import com.intellij.openapi.components.RoamingType;
import com.intellij.openapi.components.Service;
import com.intellij.openapi.components.State;
import com.intellij.openapi.components.Storage;
import org.jetbrains.annotations.NotNull;

/** The binary to run instead of the bundled one, per IDE and never synced, as paths differ. */
@Service(Service.Level.APP)
@State(
    name = "adocfmt",
    storages = @Storage(value = "adocfmt.xml", roamingType = RoamingType.DISABLED))
public final class AdocfmtIdeSettings
    implements PersistentStateComponent<AdocfmtIdeSettings.Fields> {
  public static final class Fields {
    public String binaryPath = "";
  }

  private Fields fields = new Fields();

  static AdocfmtIdeSettings getInstance() {
    return ApplicationManager.getApplication().getService(AdocfmtIdeSettings.class);
  }

  String getBinaryPath() {
    return fields.binaryPath;
  }

  void setBinaryPath(String binaryPath) {
    fields.binaryPath = binaryPath;
  }

  @Override
  public @NotNull Fields getState() {
    return fields;
  }

  @Override
  public void loadState(@NotNull Fields fields) {
    this.fields = fields;
  }
}
