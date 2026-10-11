package io.github.ypfaff.adocfmt.intellij;

import com.intellij.openapi.components.PersistentStateComponent;
import com.intellij.openapi.components.Service;
import com.intellij.openapi.components.State;
import com.intellij.openapi.components.Storage;
import com.intellij.openapi.project.Project;
import org.jetbrains.annotations.NotNull;

/** Whether adocfmt formats the project, kept in .idea/adocfmt.xml so a team can commit it. */
@Service(Service.Level.PROJECT)
@State(name = "adocfmt", storages = @Storage("adocfmt.xml"))
public final class AdocfmtSettings implements PersistentStateComponent<AdocfmtSettings.Fields> {
  public static final class Fields {
    public boolean enabled;
  }

  private Fields fields = new Fields();

  static AdocfmtSettings getInstance(Project project) {
    return project.getService(AdocfmtSettings.class);
  }

  boolean isEnabled() {
    return fields.enabled;
  }

  void setEnabled(boolean enabled) {
    fields.enabled = enabled;
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
