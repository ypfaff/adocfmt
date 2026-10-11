package io.github.ypfaff.adocfmt.intellij;

import com.intellij.openapi.options.Configurable;
import com.intellij.openapi.project.Project;
import com.intellij.ui.components.JBCheckBox;
import java.awt.BorderLayout;
import javax.swing.JComponent;
import javax.swing.JPanel;

/** The page under Tools › adocfmt. */
final class AdocfmtConfigurable implements Configurable {
  private final AdocfmtSettings settings;
  private final JBCheckBox enabled =
      new JBCheckBox("Format AsciiDoc files in this project with adocfmt");

  AdocfmtConfigurable(Project project) {
    settings = AdocfmtSettings.getInstance(project);
  }

  @Override
  public String getDisplayName() {
    return "adocfmt";
  }

  @Override
  public JComponent createComponent() {
    JPanel page = new JPanel(new BorderLayout());
    page.add(enabled, BorderLayout.NORTH);
    return page;
  }

  @Override
  public boolean isModified() {
    return enabled.isSelected() != settings.isEnabled();
  }

  @Override
  public void apply() {
    settings.setEnabled(enabled.isSelected());
  }

  @Override
  public void reset() {
    enabled.setSelected(settings.isEnabled());
  }
}
